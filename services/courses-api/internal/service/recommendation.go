package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

var (
	ErrQuestionRequired           = errors.New("question is required")
	ErrRecommendationsDisabled    = errors.New("course recommendations are not configured")
	ErrRecommendationCatalogEmpty = errors.New("no published courses available for recommendation")
)

type RecommendationCatalogItem struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	ShortDescription string   `json:"shortDescription"`
	Level            string   `json:"level"`
	Category         string   `json:"category"`
	Price            float64  `json:"price"`
	Currency         string   `json:"currency"`
	Capacity         int      `json:"capacity,omitempty"`
	AvailableSeats   int      `json:"availableSeats,omitempty"`
	Teachers         []string `json:"teachers,omitempty"`
	DurationLabel    string   `json:"duration,omitempty"`
}

type RecommendationCatalogRepository interface {
	ListRecommendationCatalog(ctx context.Context) ([]RecommendationCatalogItem, error)
}

type RecommendationTeacherLookup interface {
	ListTeachers(ctx context.Context, courseID string) ([]string, error)
}

type RecommendationTeacherResolver interface {
	GetTeacher(ctx context.Context, teacherID string) (*TeacherInfo, error)
}

type RecommendationAvailability struct {
	CourseID    string
	Capacity    int
	ActiveCount int
	Available   int
}

type RecommendationAvailabilityLookup interface {
	GetAvailability(ctx context.Context, courseID string) (*RecommendationAvailability, error)
}

type RecommendationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RecommendationAdvisor interface {
	Recommend(ctx context.Context, systemPrompt string, history []RecommendationMessage, question string) (string, error)
}

type RecommendationResult struct {
	Answer  string
	Catalog []RecommendationCatalogItem
}

type RecommendationResponseOptions struct {
	UserName string
	History  []RecommendationMessage
}

type RecommendationService struct {
	catalog      RecommendationCatalogRepository
	teacherRepo  RecommendationTeacherLookup
	teachers     RecommendationTeacherResolver
	availability RecommendationAvailabilityLookup
	advisor      RecommendationAdvisor
}

const maxRecommendationHistoryMessages = 8

func NewRecommendationService(
	catalog RecommendationCatalogRepository,
	teacherRepo RecommendationTeacherLookup,
	teachers RecommendationTeacherResolver,
	availability RecommendationAvailabilityLookup,
	advisor RecommendationAdvisor,
) *RecommendationService {
	return &RecommendationService{
		catalog:      catalog,
		teacherRepo:  teacherRepo,
		teachers:     teachers,
		availability: availability,
		advisor:      advisor,
	}
}

func (s *RecommendationService) Recommend(ctx context.Context, userName, question string, history []RecommendationMessage) (*RecommendationResult, error) {
	trimmedQuestion := strings.TrimSpace(question)
	if trimmedQuestion == "" {
		return nil, ErrQuestionRequired
	}
	if s.catalog == nil {
		return nil, ErrRecommendationsDisabled
	}

	catalog, err := s.catalog.ListRecommendationCatalog(ctx)
	if err != nil {
		return nil, err
	}
	catalog = s.enrichRecommendationCatalog(ctx, catalog)
	if len(catalog) == 0 {
		return &RecommendationResult{
			Answer:  "Uy, justo ahora no veo cursos publicados para recomendarte. Si queres, lo revisamos de nuevo en un rato y vemos que aparece.",
			Catalog: nil,
		}, nil
	}

	normalizedHistory := normalizeRecommendationHistory(history)

	if s.advisor != nil {
		advisorHistory := buildRecommendationAdvisorHistory(trimmedQuestion, normalizedHistory)
		advisorCatalog := selectRecommendationAdvisorCatalog(trimmedQuestion, advisorHistory, catalog)
		systemPrompt, promptErr := BuildRecommendationSystemPrompt(
			advisorCatalog,
			userName,
			advisorHistory,
			trimmedQuestion,
			buildRecommendationAdvisorContextSummary(trimmedQuestion, advisorHistory),
		)
		if promptErr == nil {
			answer, advisorErr := s.advisor.Recommend(ctx, systemPrompt, advisorHistory, trimmedQuestion)
			if advisorErr == nil && strings.TrimSpace(answer) != "" {
				return &RecommendationResult{
					Answer:  strings.TrimSpace(answer),
					Catalog: catalog,
				}, nil
			}
		}
	}

	return &RecommendationResult{
		Answer: BuildFallbackRecommendation(trimmedQuestion, catalog, RecommendationResponseOptions{
			UserName: userName,
			History:  normalizedHistory,
		}),
		Catalog: catalog,
	}, nil
}

func (s *RecommendationService) enrichRecommendationCatalog(ctx context.Context, catalog []RecommendationCatalogItem) []RecommendationCatalogItem {
	if len(catalog) == 0 {
		return catalog
	}

	enriched := make([]RecommendationCatalogItem, 0, len(catalog))
	teacherNameCache := make(map[string]string)

	for _, item := range catalog {
		enrichedItem := item

		if s.teacherRepo != nil && s.teachers != nil {
			teacherIDs, err := s.teacherRepo.ListTeachers(ctx, item.ID)
			if err == nil && len(teacherIDs) > 0 {
				teacherNames := make([]string, 0, len(teacherIDs))
				for _, teacherID := range teacherIDs {
					name, ok := teacherNameCache[teacherID]
					if !ok {
						teacher, err := s.teachers.GetTeacher(ctx, teacherID)
						if err != nil || teacher == nil || strings.TrimSpace(teacher.Name) == "" {
							continue
						}
						name = strings.TrimSpace(teacher.Name)
						teacherNameCache[teacherID] = name
					}
					teacherNames = append(teacherNames, name)
				}
				if len(teacherNames) > 0 {
					enrichedItem.Teachers = teacherNames
				}
			}
		}

		if s.availability != nil {
			availability, err := s.availability.GetAvailability(ctx, item.ID)
			if err == nil && availability != nil {
				if availability.Capacity > 0 {
					enrichedItem.Capacity = availability.Capacity
				}
				if availability.Available >= 0 {
					enrichedItem.AvailableSeats = availability.Available
				}
			}
		}

		enriched = append(enriched, enrichedItem)
	}

	return enriched
}

func BuildRecommendationSystemPrompt(
	catalog []RecommendationCatalogItem,
	userName string,
	history []RecommendationMessage,
	question string,
	contextSummary string,
) (string, error) {
	catalogJSON, err := FormatRecommendationCatalog(catalog)
	if err != nil {
		return "", err
	}

	conversationStageInstruction := "Ya hubo intercambio previo en la conversacion. No repitas el saludo inicial."
	if len(history) == 0 {
		conversationStageInstruction = "Todavia no hubo respuestas tuyas en esta conversacion. Debes cumplir el saludo inicial exacto antes de cualquier otra cosa."
	}

	parts := []string{
		"Sos el asistente oficial de la plataforma de cursos.",
		"Tu rol es funcionar como una secretaria academica inteligente, amable y cercana.",
		"Hablas como una persona experta en educacion que conversa de forma natural, clara y humana.",
		"Nunca digas que sos una IA, nunca suenes como robot y nunca respondas como soporte tecnico.",
		"Tu objetivo principal es ayudar a descubrir que estudiar dentro de la plataforma.",
		"Tu segundo objetivo es motivar al usuario a avanzar dentro de la plataforma.",
		"Quiero que la conversacion se sienta natural, calida y fluida, como hablar con una persona de una academia.",
		"Usa frases cortas, cercanas y conversacionales. Evita bloques largos, plantillas repetidas y listas mecanicas.",
		"Cuando recomiendes cursos, usa listas cortas con 2 o 3 opciones como maximo y explica brevemente por que sirve cada una.",
		"Si aun no hay suficiente contexto, hace solo 1 o 2 preguntas utiles para entender nivel, objetivo o formato deseado.",
		"Si el usuario esta perdido, ayudalo con preguntas simples como tecnico, creativo o negocios.",
		"Si el usuario ya te dio contexto suficiente, recomienda directamente usando solo cursos publicados del catalogo real.",
		"Si la persona pregunta por profesores, docentes, cupos, precio, costo, duracion o tiempo, responde usando esos datos del catalogo si estan disponibles.",
		"Si la duracion no esta disponible en el catalogo, dilo con naturalidad y no inventes horas ni semanas.",
		"Debes guiar siempre la conversacion hacia estudiar algo util dentro del catalogo disponible.",
		"Basa tus respuestas UNICAMENTE en el catalogo provisto.",
		"No inventes cursos, niveles, duraciones, precios, profesores ni beneficios.",
		"Si el catalogo no trae un dato, no lo menciones.",
		"Si la persona marca un presupuesto o un tope de precio, respetalo estrictamente.",
		"Si existen cursos claramente alineados con el tema pedido, no menciones areas no relacionadas solo para completar.",
		"Prioriza coincidencias tematicas claras: derecho para consultas juridicas, finanzas para presupuesto e inversion, y tecnologia para programacion o software.",
		"Ten muy en cuenta el contexto reciente de la conversacion antes de responder.",
		"La ultima consulta del usuario tiene prioridad. Si la persona se corrige, cambia de idea o dice que se confundio, descarta el tema anterior y responde segun el tema nuevo.",
		"Si el usuario rechaza una opcion previa o dice que no le interesa, no insistas con esa misma recomendacion y propon una alternativa distinta del catalogo.",
		"No copies el catalogo como una ficha tecnica. Integra esos datos dentro de una respuesta natural.",
		"Si solo conoces algunos cursos relevantes, habla sobre esos y no rellenes con opciones fuera de tema.",
		conversationStageInstruction,
		buildRecommendationInitialInstruction(userName),
		"Consulta actual prioritaria del usuario:",
		question,
		"Contexto util confirmado de la conversacion:",
		contextSummary,
		"Catalogo de cursos disponible:",
		catalogJSON,
	}

	return strings.Join(parts, "\n"), nil
}

func FormatRecommendationCatalog(catalog []RecommendationCatalogItem) (string, error) {
	payload, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return "", err
	}

	return string(payload), nil
}

func normalizeRecommendationHistory(history []RecommendationMessage) []RecommendationMessage {
	if len(history) == 0 {
		return nil
	}

	normalized := make([]RecommendationMessage, 0, len(history))
	for _, message := range history {
		role := normalizeRecommendationMessageRole(message.Role)
		content := strings.TrimSpace(message.Content)
		if role == "" || content == "" {
			continue
		}

		normalized = append(normalized, RecommendationMessage{
			Role:    role,
			Content: content,
		})
	}

	if len(normalized) > maxRecommendationHistoryMessages {
		normalized = normalized[len(normalized)-maxRecommendationHistoryMessages:]
	}

	return normalized
}

func normalizeRecommendationMessageRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "user":
		return "user"
	case "assistant":
		return "assistant"
	default:
		return ""
	}
}

func buildRecommendationFallbackQuestion(question string, history []RecommendationMessage) string {
	currentQuestion := strings.TrimSpace(question)
	if currentQuestion == "" {
		return ""
	}

	userMessages := recommendationHistoryUserMessages(history)
	if len(userMessages) == 0 {
		return currentQuestion
	}

	currentTokens := tokenizeRecommendationText(currentQuestion)
	currentSignals := detectRecommendationContextSignals(currentQuestion, currentTokens)
	currentDomain := primaryRecommendationDomain(currentQuestion)
	preferCurrentTopic := shouldPreferCurrentRecommendationTopic(currentQuestion, history)

	supportingContext := make([]string, 0, 2)
	for i := len(userMessages) - 1; i >= 0 && len(supportingContext) < 2; i-- {
		content := strings.TrimSpace(userMessages[i])
		if content == "" {
			continue
		}

		historyTokens := tokenizeRecommendationText(content)
		historySignals := detectRecommendationContextSignals(content, historyTokens)
		historyDomain := primaryRecommendationDomain(content)

		if preferCurrentTopic {
			if currentDomain != "" && historyDomain != "" && historyDomain != currentDomain {
				continue
			}
			if currentSignals.HasTopic && historySignals.HasTopic && currentDomain == "" &&
				!recommendationMessagesShareTopic(currentTokens, historyTokens) {
				continue
			}
		}

		contributes := false
		if !currentSignals.HasTopic && historySignals.HasTopic {
			contributes = true
		}
		if !currentSignals.HasExperience && historySignals.HasExperience {
			contributes = true
			currentSignals.HasExperience = true
		}
		if !currentSignals.HasGoal && historySignals.HasGoal {
			contributes = true
			currentSignals.HasGoal = true
		}
		if !currentSignals.HasFormat && historySignals.HasFormat {
			contributes = true
			currentSignals.HasFormat = true
		}
		if !contributes {
			continue
		}

		supportingContext = append(supportingContext, content)
	}

	parts := make([]string, 0, len(supportingContext)+1)
	for i := len(supportingContext) - 1; i >= 0; i-- {
		parts = append(parts, supportingContext[i])
	}
	parts = append(parts, currentQuestion)

	return strings.Join(parts, "\n")
}

func buildRecommendationAdvisorHistory(question string, history []RecommendationMessage) []RecommendationMessage {
	if len(history) == 0 || !shouldPreferCurrentRecommendationTopic(question, history) {
		return history
	}

	currentTokens := tokenizeRecommendationText(question)
	currentDomain := primaryRecommendationDomain(question)
	filtered := make([]RecommendationMessage, 0, len(history))
	keepAssistantReply := false

	for _, message := range history {
		switch message.Role {
		case "user":
			keepAssistantReply = recommendationMessageSupportsCurrentTopic(message.Content, currentTokens, currentDomain)
			if keepAssistantReply {
				filtered = append(filtered, message)
			}
		case "assistant":
			if keepAssistantReply {
				filtered = append(filtered, message)
			}
		}
	}

	if len(filtered) == 0 {
		return nil
	}

	return filtered
}

func recommendationMessageSupportsCurrentTopic(message string, currentTokens []string, currentDomain string) bool {
	historyTokens := tokenizeRecommendationText(message)
	historySignals := detectRecommendationContextSignals(message, historyTokens)
	historyDomain := primaryRecommendationDomain(message)

	if !hasSpecificTopicTokens(historyTokens) {
		return historySignals.HasExperience || historySignals.HasGoal || historySignals.HasFormat
	}
	if currentDomain != "" && historyDomain != "" {
		return currentDomain == historyDomain
	}

	return recommendationMessagesShareTopic(currentTokens, historyTokens)
}

func buildRecommendationAdvisorContextSummary(question string, history []RecommendationMessage) string {
	contextMessages := make([]string, 0, 3)
	for _, message := range history {
		if message.Role != "user" {
			continue
		}

		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}

		contextMessages = append(contextMessages, content)
		if len(contextMessages) == 3 {
			break
		}
	}

	if len(contextMessages) == 0 {
		return "Sin contexto adicional confirmado mas alla de la consulta actual."
	}

	return strings.Join(contextMessages, " | ")
}

func selectRecommendationAdvisorCatalog(question string, history []RecommendationMessage, catalog []RecommendationCatalogItem) []RecommendationCatalogItem {
	currentQuestion := strings.TrimSpace(question)
	contextualQuestion := buildRecommendationFallbackQuestion(currentQuestion, history)
	targetQuestion := contextualQuestion
	if shouldPreferCurrentRecommendationTopic(currentQuestion, history) || asksForExplanation(currentQuestion) || shouldUseCurrentQuestionAsPrimarySignal(currentQuestion) {
		targetQuestion = currentQuestion
	}

	filteredCatalog := filterCatalogByBudget(targetQuestion, catalog)
	if len(filteredCatalog) == 0 {
		filteredCatalog = catalog
	}
	filteredCatalog = filterRecommendationCatalogByRejectedOptions(currentQuestion, history, filteredCatalog)
	if len(filteredCatalog) == 0 {
		filteredCatalog = filterCatalogByBudget(targetQuestion, catalog)
		if len(filteredCatalog) == 0 {
			filteredCatalog = catalog
		}
	}

	targetTokens := tokenizeRecommendationText(targetQuestion)
	if !hasSpecificTopicTokens(targetTokens) && !asksForPrices(targetQuestion) && !asksForTeachers(targetQuestion) &&
		!asksForAvailableSeats(targetQuestion) && !asksForDuration(targetQuestion) {
		if len(filteredCatalog) > 8 {
			return append([]RecommendationCatalogItem(nil), filteredCatalog[:8]...)
		}
		return filteredCatalog
	}

	ranked := filterRankedByPrimaryDomain(targetQuestion, rankCatalog(targetQuestion, filteredCatalog))
	selected := make([]RecommendationCatalogItem, 0, 6)
	for _, candidate := range ranked {
		if candidate.score <= 0 && len(selected) >= 3 {
			break
		}
		selected = append(selected, candidate.item)
		if len(selected) == 6 {
			break
		}
	}

	if len(selected) == 0 {
		if len(filteredCatalog) > 8 {
			return append([]RecommendationCatalogItem(nil), filteredCatalog[:8]...)
		}
		return filteredCatalog
	}

	return selected
}

func shouldUseCurrentQuestionAsPrimarySignal(question string) bool {
	return asksForTeachers(question) || asksForAvailableSeats(question) || asksForDuration(question) ||
		asksForPrices(question) || asksForExplanation(question)
}

func filterRecommendationCatalogByRejectedOptions(question string, history []RecommendationMessage, catalog []RecommendationCatalogItem) []RecommendationCatalogItem {
	rejected := extractRejectedRecommendationTitles(question, history, catalog)
	if len(rejected) == 0 {
		return catalog
	}

	filtered := make([]RecommendationCatalogItem, 0, len(catalog))
	for _, item := range catalog {
		if _, excluded := rejected[normalizeRecommendationText(item.Title)]; excluded {
			continue
		}
		filtered = append(filtered, item)
	}

	return filtered
}

func extractRejectedRecommendationTitles(question string, history []RecommendationMessage, catalog []RecommendationCatalogItem) map[string]struct{} {
	if !rejectsPreviousRecommendation(question) {
		return nil
	}

	rejected := make(map[string]struct{})
	normalizedQuestion := normalizeRecommendationText(question)

	for _, item := range catalog {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}
		if strings.Contains(normalizedQuestion, normalizeRecommendationText(title)) {
			rejected[normalizeRecommendationText(title)] = struct{}{}
		}
	}

	position := rejectedRecommendationPosition(question)
	if position == 0 {
		return rejected
	}

	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role != "assistant" {
			continue
		}

		titles := extractRecommendationTitlesFromMessage(history[i].Content, catalog)
		if len(titles) == 0 {
			continue
		}
		index := position - 1
		if index < 0 || index >= len(titles) {
			return rejected
		}
		rejected[normalizeRecommendationText(titles[index])] = struct{}{}
		return rejected
	}

	return rejected
}

func rejectsPreviousRecommendation(question string) bool {
	normalized := normalizeRecommendationText(question)
	rejectionMarkers := []string{
		"no me interesa",
		"no me interesan",
		"no me gusto",
		"no me gustan",
		"no quiero ese",
		"no quiero esa",
		"no quiero eso",
		"no quiero lo primero",
		"descarta",
		"descartemos",
		"dejemos",
		"prefiero otra opcion",
		"quiero otra opcion",
		"otra opcion",
		"algo distinto",
		"otra cosa",
		"ese no",
		"esa no",
		"eso no",
	}
	for _, marker := range rejectionMarkers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}

	return false
}

func rejectedRecommendationPosition(question string) int {
	normalized := normalizeRecommendationText(question)
	switch {
	case strings.Contains(normalized, "lo primero"), strings.Contains(normalized, "la primera"), strings.Contains(normalized, "primer curso"), strings.Contains(normalized, "primera opcion"):
		return 1
	case strings.Contains(normalized, "lo segundo"), strings.Contains(normalized, "la segunda"), strings.Contains(normalized, "segundo curso"), strings.Contains(normalized, "segunda opcion"):
		return 2
	case strings.Contains(normalized, "lo tercero"), strings.Contains(normalized, "la tercera"), strings.Contains(normalized, "tercer curso"), strings.Contains(normalized, "tercera opcion"), strings.Contains(normalized, "lo ultimo"), strings.Contains(normalized, "la ultima"):
		return 3
	default:
		return 0
	}
}

func extractRecommendationTitlesFromMessage(message string, catalog []RecommendationCatalogItem) []string {
	normalizedMessage := normalizeRecommendationText(message)
	type titleHit struct {
		title string
		index int
	}

	hits := make([]titleHit, 0, len(catalog))
	seen := make(map[string]struct{}, len(catalog))
	for _, item := range catalog {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}
		normalizedTitle := normalizeRecommendationText(title)
		index := strings.Index(normalizedMessage, normalizedTitle)
		if index < 0 {
			continue
		}
		if _, exists := seen[normalizedTitle]; exists {
			continue
		}
		seen[normalizedTitle] = struct{}{}
		hits = append(hits, titleHit{title: title, index: index})
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].index == hits[j].index {
			return hits[i].title < hits[j].title
		}
		return hits[i].index < hits[j].index
	})

	titles := make([]string, 0, len(hits))
	for _, hit := range hits {
		titles = append(titles, hit.title)
	}

	return titles
}

func recommendationHistoryUserMessages(history []RecommendationMessage) []string {
	userMessages := make([]string, 0, len(history))
	for _, message := range history {
		if message.Role != "user" {
			continue
		}

		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}

		userMessages = append(userMessages, content)
	}

	if len(userMessages) > 4 {
		userMessages = userMessages[len(userMessages)-4:]
	}

	return userMessages
}

func shouldPreferCurrentRecommendationTopic(question string, history []RecommendationMessage) bool {
	normalizedQuestion := normalizeRecommendationText(question)
	if normalizedQuestion == "" {
		return false
	}

	currentTokens := tokenizeRecommendationText(question)
	currentDomain := primaryRecommendationDomain(question)
	if rejectsPreviousRecommendation(question) && currentDomain == "" && !hasSpecificTopicTokens(currentTokens) {
		return false
	}

	overrideMarkers := []string{
		"me confundi",
		"en realidad",
		"al final",
		"cambie de idea",
		"cambio de idea",
		"ahora quiero",
		"mejor quiero",
		"quise decir",
		"quise poner",
		"no queria",
		"no, queria",
	}
	for _, marker := range overrideMarkers {
		if strings.Contains(normalizedQuestion, marker) {
			return true
		}
	}

	if currentDomain == "" && !hasSpecificTopicTokens(currentTokens) {
		return false
	}

	userMessages := recommendationHistoryUserMessages(history)
	for i := len(userMessages) - 1; i >= 0; i-- {
		historyQuestion := userMessages[i]
		historyDomain := primaryRecommendationDomain(historyQuestion)
		historyTokens := tokenizeRecommendationText(historyQuestion)

		if currentDomain != "" && historyDomain != "" && historyDomain != currentDomain {
			return true
		}
		if currentDomain == "" && hasSpecificTopicTokens(historyTokens) &&
			!recommendationMessagesShareTopic(currentTokens, historyTokens) {
			return true
		}
	}

	return false
}

func recommendationMessagesShareTopic(left, right []string) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}

	variants := make(map[string]struct{}, len(left)*2)
	for _, token := range left {
		for _, variant := range recommendationTokenVariants(token) {
			variants[variant] = struct{}{}
		}
	}

	for _, token := range right {
		for _, variant := range recommendationTokenVariants(token) {
			if _, ok := variants[variant]; ok {
				return true
			}
		}
	}

	return false
}

type rankedRecommendation struct {
	item  RecommendationCatalogItem
	score int
}

func BuildFallbackRecommendation(question string, catalog []RecommendationCatalogItem, options RecommendationResponseOptions) string {
	currentQuestion := strings.TrimSpace(question)
	fullQuestion := buildRecommendationFallbackQuestion(currentQuestion, options.History)
	currentTokens := tokenizeRecommendationText(currentQuestion)
	fullTokens := tokenizeRecommendationText(fullQuestion)

	if metadataAnswer := buildRecommendationMetadataResponse(currentQuestion, catalog); metadataAnswer != "" {
		return metadataAnswer
	}
	if explanationAnswer := buildRecommendationExplanationResponse(currentQuestion, fullQuestion, catalog); explanationAnswer != "" {
		return explanationAnswer
	}
	if len(options.History) == 0 {
		return buildInitialClarifyingRecommendation(options.UserName, currentQuestion)
	}
	if needsRecommendationClarification(currentQuestion, currentTokens) &&
		needsRecommendationClarification(fullQuestion, fullTokens) {
		return buildContextClarifyingRecommendation(currentQuestion)
	}

	filteredCatalog := filterCatalogByBudget(fullQuestion, catalog)
	filteredCatalog = filterRecommendationCatalogByRejectedOptions(currentQuestion, options.History, filteredCatalog)
	if len(filteredCatalog) == 0 {
		filteredCatalog = filterCatalogByBudget(fullQuestion, catalog)
	}
	if budget, ok := extractRecommendationMaxBudget(fullQuestion); ok && !hasSpecificTopicTokens(fullTokens) {
		return buildBudgetRecommendation(filteredCatalog, budget, recommendationStyleVariant(question, "budget"))
	}
	ranked := rankCatalog(fullQuestion, filteredCatalog)
	ranked = filterRankedByPrimaryDomain(fullQuestion, ranked)
	if len(ranked) == 0 {
		if budget, ok := extractRecommendationMaxBudget(fullQuestion); ok {
			return buildBudgetUnavailableRecommendation(budget)
		}
		return "Ahora mismo no veo cursos publicados que encajen con eso. Si queres, contame un poco mas y lo pensamos juntos."
	}

	top := ranked[0]
	variant := recommendationStyleVariant(question, top.item.Title)
	if budget, ok := extractRecommendationMaxBudget(question); ok && top.score <= 0 {
		return buildBudgetUnavailableRecommendation(budget)
	}
	if top.score <= 0 {
		return buildLooseRecommendation(question, top.item, variant)
	}

	limit := 3
	if len(ranked) < limit {
		limit = len(ranked)
	}

	items := make([]RecommendationCatalogItem, 0, limit)
	for _, candidate := range ranked[:limit] {
		if candidate.score <= 0 {
			break
		}

		items = append(items, candidate.item)
	}

	if len(items) == 0 {
		items = append(items, top.item)
	}

	return buildStructuredRecommendation(currentQuestion, items, variant)
}

func buildRecommendationMetadataResponse(question string, catalog []RecommendationCatalogItem) string {
	switch {
	case asksForTeachers(question):
		return buildTeacherMetadataRecommendation(question, catalog)
	case asksForAvailableSeats(question):
		return buildSeatsMetadataRecommendation(question, catalog)
	case asksForDuration(question):
		return buildDurationMetadataRecommendation(question, catalog)
	case asksForPrices(question):
		return buildPriceMetadataRecommendation(question, catalog)
	default:
		return ""
	}
}

func buildRecommendationExplanationResponse(currentQuestion, contextualQuestion string, catalog []RecommendationCatalogItem) string {
	if !asksForExplanation(currentQuestion) {
		return ""
	}

	ranked := rankCatalog(currentQuestion, catalog)
	ranked = filterRankedByPrimaryDomain(currentQuestion, ranked)
	if len(ranked) == 0 || ranked[0].score <= 0 {
		ranked = rankCatalog(contextualQuestion, catalog)
		ranked = filterRankedByPrimaryDomain(contextualQuestion, ranked)
	}
	if len(ranked) == 0 || ranked[0].score <= 0 {
		return ""
	}

	item := ranked[0].item
	parts := []string{
		fmt.Sprintf("%s va por este lado:", item.Title),
		"",
		buildRecommendationExplanationSentence(item),
	}

	if len(item.Teachers) > 0 {
		parts = append(parts, fmt.Sprintf("Hoy lo esta dando %s.", joinRecommendationCategories(item.Teachers)))
	}
	if item.Price > 0 && strings.TrimSpace(item.Currency) != "" {
		parts = append(parts, fmt.Sprintf("Ahora mismo figura con un precio de %.0f %s.", item.Price, strings.TrimSpace(item.Currency)))
	}
	if item.AvailableSeats > 0 && item.Capacity > 0 {
		parts = append(parts, fmt.Sprintf("Ademas, todavia tiene %d cupos disponibles.", item.AvailableSeats))
	}

	parts = append(parts, "", "Si queres, tambien te cuento si te conviene para empezar o si primero miraria otro curso.")
	return strings.Join(parts, "\n")
}

func asksForExplanation(question string) bool {
	normalized := normalizeRecommendationText(question)
	return strings.Contains(normalized, "de que se trata") ||
		strings.Contains(normalized, "explicame") ||
		strings.Contains(normalized, "explicame de que se trata") ||
		strings.Contains(normalized, "contame sobre") ||
		strings.Contains(normalized, "quiero saber sobre")
}

func buildRecommendationExplanationReason(item RecommendationCatalogItem) string {
	if description := recommendationDescriptionClause(item.ShortDescription); description != "" {
		return description
	}

	if strings.TrimSpace(item.Category) != "" {
		return "entender mejor el area de " + strings.TrimSpace(item.Category)
	}

	return "entender mejor ese tema"
}

func buildRecommendationExplanationSentence(item RecommendationCatalogItem) string {
	reason := strings.TrimSpace(buildRecommendationExplanationReason(item))
	if reason == "" {
		return "Te ayuda a entender mejor de que se trata."
	}

	lowered := strings.ToLower(reason)
	if strings.HasPrefix(lowered, "te ") || strings.HasPrefix(lowered, "esta ") || strings.HasPrefix(lowered, "va ") {
		return upperFirst(strings.TrimSuffix(reason, ".")) + "."
	}

	return "Te ayuda a " + strings.TrimSuffix(reason, ".") + "."
}

func asksForTeachers(question string) bool {
	normalized := normalizeRecommendationText(question)
	return strings.Contains(normalized, "profesor") ||
		strings.Contains(normalized, "docente") ||
		strings.Contains(normalized, "quien da") ||
		strings.Contains(normalized, "quienes dan") ||
		strings.Contains(normalized, "dando clases")
}

func asksForAvailableSeats(question string) bool {
	normalized := normalizeRecommendationText(question)
	return strings.Contains(normalized, "cupo") ||
		strings.Contains(normalized, "vacante") ||
		strings.Contains(normalized, "lugar disponible") ||
		strings.Contains(normalized, "quedan lugares")
}

func asksForDuration(question string) bool {
	normalized := normalizeRecommendationText(question)
	return strings.Contains(normalized, "duracion") ||
		strings.Contains(normalized, "cuanto dura") ||
		strings.Contains(normalized, "tiempo") ||
		strings.Contains(normalized, "cuantas horas")
}

func asksForPrices(question string) bool {
	normalized := normalizeRecommendationText(question)
	return strings.Contains(normalized, "precio") ||
		strings.Contains(normalized, "cuesta") ||
		strings.Contains(normalized, "costa") ||
		strings.Contains(normalized, "sale") ||
		strings.Contains(normalized, "valor")
}

func buildTeacherMetadataRecommendation(question string, catalog []RecommendationCatalogItem) string {
	ranked := filterRankedByPrimaryDomain(question, rankCatalog(question, catalog))

	type teacherEntry struct {
		title    string
		teachers []string
	}

	entries := make([]teacherEntry, 0, len(ranked))
	for _, candidate := range ranked {
		if len(candidate.item.Teachers) == 0 {
			continue
		}
		entries = append(entries, teacherEntry{
			title:    candidate.item.Title,
			teachers: candidate.item.Teachers,
		})
		if len(entries) == 4 {
			break
		}
	}

	if len(entries) == 0 {
		return "Por ahora no tengo profesores publicados dentro del catalogo para mostrarte con precision. Si queres, igual te puedo recomendar cursos por tema, precio o cupos."
	}

	lines := []string{"Hoy estos cursos ya tienen profesores asignados:", ""}
	for _, entry := range entries {
		lines = append(lines, fmt.Sprintf("\u2022 %s: %s.", entry.title, joinRecommendationCategories(entry.teachers)))
	}
	lines = append(lines, "", "Si queres, tambien te digo cual de esos te conviene mirar primero.")
	return strings.Join(lines, "\n")
}

func buildSeatsMetadataRecommendation(question string, catalog []RecommendationCatalogItem) string {
	ranked := filterRankedByPrimaryDomain(question, rankCatalog(question, catalog))
	items := make([]RecommendationCatalogItem, 0, len(ranked))
	for _, candidate := range ranked {
		if candidate.item.Capacity <= 0 {
			continue
		}
		items = append(items, candidate.item)
	}
	if len(items) == 0 {
		return "Todavia no veo cupos publicados para mostrarte con precision. Si queres, te ayudo igual a elegir por tema o por precio."
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].AvailableSeats == items[j].AvailableSeats {
			return items[i].Title < items[j].Title
		}
		return items[i].AvailableSeats > items[j].AvailableSeats
	})
	if len(items) > 4 {
		items = items[:4]
	}

	lines := []string{"Te paso los cursos con cupos visibles en este momento:", ""}
	for _, item := range items {
		if item.AvailableSeats > 0 {
			lines = append(lines, fmt.Sprintf("\u2022 %s: quedan %d cupos de %d.", item.Title, item.AvailableSeats, item.Capacity))
			continue
		}
		lines = append(lines, fmt.Sprintf("\u2022 %s: ahora mismo no tiene cupos disponibles sobre %d lugares.", item.Title, item.Capacity))
	}
	lines = append(lines, "", "Si queres, tambien te filtro solo los que todavia tienen lugar.")
	return strings.Join(lines, "\n")
}

func buildDurationMetadataRecommendation(question string, catalog []RecommendationCatalogItem) string {
	ranked := filterRankedByPrimaryDomain(question, rankCatalog(question, catalog))
	lines := []string{}
	for _, candidate := range ranked {
		if strings.TrimSpace(candidate.item.DurationLabel) == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("\u2022 %s: %s.", candidate.item.Title, candidate.item.DurationLabel))
		if len(lines) == 4 {
			break
		}
	}
	if len(lines) == 0 {
		return "Todavia no tengo la duracion publicada de los cursos dentro del catalogo. Si queres, igual te puedo orientar por precio, profesores, cupos o por que tan completo parece cada curso."
	}

	return "Estas son las duraciones que hoy tengo publicadas:\n\n" + strings.Join(lines, "\n")
}

func buildPriceMetadataRecommendation(question string, catalog []RecommendationCatalogItem) string {
	ranked := filterRankedByPrimaryDomain(question, rankCatalog(question, catalog))
	items := make([]RecommendationCatalogItem, 0, len(ranked))
	for _, candidate := range ranked {
		items = append(items, candidate.item)
	}
	if len(items) == 0 {
		items = append(items, catalog...)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Price == items[j].Price {
			return items[i].Title < items[j].Title
		}
		return items[i].Price < items[j].Price
	})
	if len(items) > 4 {
		items = items[:4]
	}

	lines := []string{"Estos son los precios que hoy veo en el catalogo:", ""}
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("\u2022 %s: %.0f %s.", item.Title, item.Price, strings.TrimSpace(item.Currency)))
	}
	lines = append(lines, "", "Si queres, tambien te los ordeno por mas barato, mas completo o por categoria.")
	return strings.Join(lines, "\n")
}

type recommendationContextSignals struct {
	HasTopic      bool
	HasExperience bool
	HasGoal       bool
	HasFormat     bool
}

func buildRecommendationInitialInstruction(userName string) string {
	return "Si es tu primera respuesta en esta conversacion, debes abrir exactamente asi:\n" +
		buildRecommendationInitialGreeting(userName)
}

func buildRecommendationInitialGreeting(userName string) string {
	name := firstRecommendationName(userName)
	salutation := "\u00a1Hola! \U0001F44B"
	if name != "" {
		salutation = fmt.Sprintf("\u00a1Hola, %s! \U0001F44B", name)
	}

	return strings.Join([]string{
		salutation,
		"",
		"Estoy aqui para que tu experiencia con la app de cursos sea mas simple y util.",
		"",
		"Puedo ayudarte a:",
		"\u2022 elegir tu primer curso",
		"\u2022 encontrar algo que complemente lo que ya sabes",
		"\u2022 recomendarte un camino de aprendizaje",
		"",
		"Contame un poco:",
		"\u00bfque te gustaria aprender o mejorar?",
	}, "\n")
}

func firstRecommendationName(userName string) string {
	trimmed := strings.TrimSpace(userName)
	if trimmed == "" {
		return ""
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return ""
	}

	return fields[0]
}

func needsRecommendationClarification(question string, tokens []string) bool {
	if shouldAskClarifyingQuestion(question, tokens) {
		return true
	}

	signals := detectRecommendationContextSignals(question, tokens)
	if maxBudget, ok := extractRecommendationMaxBudget(question); ok && maxBudget > 0 && !signals.HasTopic {
		return false
	}
	if !signals.HasTopic {
		return true
	}

	return !(signals.HasExperience && (signals.HasGoal || signals.HasFormat))
}

func detectRecommendationContextSignals(question string, tokens []string) recommendationContextSignals {
	normalized := normalizeRecommendationText(question)
	_, hasBudget := extractRecommendationMaxBudget(question)

	experienceMarkers := []string{
		"desde cero", "principiante", "basico", "basica", "arrancando", "empiezo",
		"empezando", "intermedio", "avanzado", "ya se", "ya vi", "algo de experiencia",
		"nunca programe", "nunca estudie", "reci en empiezo", "recien empiezo",
	}
	goalMarkers := []string{
		"trabajo", "laburo", "empleo", "profesional", "hobby", "gusto", "estudio",
		"facultad", "universidad", "negocio", "emprender", "mejorar en mi trabajo",
		"cambiar de trabajo", "salida laboral",
	}
	formatMarkers := []string{
		"corto", "corta", "rapido", "rapida", "completo", "completa", "intensivo",
		"practico", "practica", "simple", "primer curso",
	}

	signals := recommendationContextSignals{
		HasTopic:  hasSpecificTopicTokens(tokens),
		HasFormat: hasBudget,
	}

	for _, marker := range experienceMarkers {
		if strings.Contains(normalized, marker) {
			signals.HasExperience = true
			break
		}
	}
	for _, marker := range goalMarkers {
		if strings.Contains(normalized, marker) {
			signals.HasGoal = true
			break
		}
	}
	for _, marker := range formatMarkers {
		if strings.Contains(normalized, marker) {
			signals.HasFormat = true
			break
		}
	}

	return signals
}

func buildInitialClarifyingRecommendation(userName, question string) string {
	parts := []string{buildRecommendationInitialGreeting(userName)}
	if discoveryLine := buildRecommendationDiscoveryLine(question); discoveryLine != "" {
		parts = append(parts, "", discoveryLine)
	}
	parts = append(parts, "", buildClarifyingQuestions(question, 2))

	return strings.Join(parts, "\n")
}

func buildContextClarifyingRecommendation(question string) string {
	return strings.Join([]string{
		"Buenisimo, ya estoy entendiendo mejor lo que buscas.",
		"",
		"Antes de recomendarte algo puntual, decime esto:",
		buildClarifyingQuestions(question, 2),
	}, "\n")
}

func buildRecommendationDiscoveryLine(question string) string {
	normalized := normalizeRecommendationText(question)
	switch {
	case strings.Contains(normalized, "program"), strings.Contains(normalized, "backend"), strings.Contains(normalized, "frontend"), strings.Contains(normalized, "web"), strings.Contains(normalized, "pagina"):
		return "Veo que te interesa algo del mundo tech, asi que podemos orientarlo bien."
	case strings.Contains(normalized, "derecho"), strings.Contains(normalized, "abog"), strings.Contains(normalized, "contrato"), strings.Contains(normalized, "laboral"):
		return "Si queres ir por un perfil juridico, hay opciones que pueden servirte mucho."
	case strings.Contains(normalized, "finanza"), strings.Contains(normalized, "ahorro"), strings.Contains(normalized, "presupuesto"), strings.Contains(normalized, "inversion"):
		return "Si tu idea va por finanzas, conviene afinar un poco el objetivo antes de elegir."
	case strings.Contains(normalized, "marketing"), strings.Contains(normalized, "redes"), strings.Contains(normalized, "ventas"):
		return "Si te interesa crecer por el lado comercial o digital, lo podemos enfocar rapido."
	default:
		return "Con un poco mas de contexto te voy a recomendar algo mucho mas util para vos."
	}
}

func buildClarifyingQuestions(question string, maxQuestions int) string {
	if maxQuestions <= 0 {
		maxQuestions = 2
	}

	tokens := tokenizeRecommendationText(question)
	signals := detectRecommendationContextSignals(question, tokens)
	questions := make([]string, 0, maxQuestions)

	if !signals.HasExperience {
		questions = append(questions, "\u2022 \u00bfEstas empezando desde cero o ya tenes algo de experiencia?")
	}
	if !signals.HasGoal && len(questions) < maxQuestions {
		questions = append(questions, "\u2022 \u00bfLo queres aprender por trabajo, hobby o estudio?")
	}
	if !signals.HasFormat && len(questions) < maxQuestions {
		questions = append(questions, "\u2022 \u00bfBuscas algo corto para arrancar o un curso mas completo?")
	}
	if !signals.HasTopic && len(questions) < maxQuestions {
		questions = append(questions, "\u2022 \u00bfTe gustaria ir por algo mas tecnico, creativo o relacionado con negocios?")
	}
	if len(questions) == 0 {
		questions = append(questions, "\u2022 \u00bfQue te gustaria priorizar primero dentro de eso?")
	}
	if len(questions) > maxQuestions {
		questions = questions[:maxQuestions]
	}

	return strings.Join(questions, "\n")
}

func buildStructuredRecommendation(question string, items []RecommendationCatalogItem, variant int) string {
	if len(items) == 0 {
		return "Ahora mismo no veo una opcion clara para recomendarte. Si queres, contame un poco mas y lo pensamos juntos."
	}

	intro := pickRecommendationPhrase([]string{
		"Buenisimo. Con lo que me contaste, estas opciones creo que te pueden servir:",
		"Con ese contexto, yo miraria primero estas opciones:",
		"Por lo que ya me contaste, aca hay cursos que encajan bastante bien:",
	}, variant)

	parts := []string{intro, ""}
	for _, item := range items {
		parts = append(parts, fmt.Sprintf("\u2022 %s. %s", item.Title, buildRecommendationBullet(item)))
	}
	parts = append(parts, "", pickRecommendationPhrase([]string{
		"Si queres, te digo por cual arrancaria yo primero.",
		"Si te sirve, tambien te las ordeno segun cual conviene mas para empezar.",
		"Si queres, vemos cual te suma mas segun el tiempo que tengas hoy.",
	}, variant))

	return strings.Join(parts, "\n")
}

func buildRecommendationBullet(item RecommendationCatalogItem) string {
	reason := strings.TrimSpace(recommendationReason(item))
	if reason == "" {
		reason = "Encaja bien con lo que estas buscando."
	} else {
		reason = upperFirst(reason)
		if !strings.HasSuffix(reason, ".") {
			reason += "."
		}
	}

	levelSentence := recommendationAudienceSentence(item.Level)
	if levelSentence == "" {
		return reason
	}

	return reason + " " + levelSentence
}

func recommendationAudienceSentence(level string) string {
	trimmed := strings.TrimSpace(level)
	normalized := normalizeRecommendationText(trimmed)
	if trimmed == "" || normalized == "no especificado" {
		return ""
	}

	return "Va bien si estas en un nivel " + trimmed + "."
}

func buildLooseRecommendation(question string, item RecommendationCatalogItem, variant int) string {
	parts := []string{
		buildRecommendationLead(question, item, variant),
		buildPrimaryRecommendation(item, variant),
		buildRecommendationFollowUp(item, variant),
	}

	return strings.Join(parts, " ")
}

func buildBudgetRecommendation(catalog []RecommendationCatalogItem, maxBudget float64, variant int) string {
	if len(catalog) == 0 {
		return buildBudgetUnavailableRecommendation(maxBudget)
	}

	items := append([]RecommendationCatalogItem(nil), catalog...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Price == items[j].Price {
			return items[i].Title < items[j].Title
		}
		return items[i].Price < items[j].Price
	})
	if len(items) > 3 {
		items = items[:3]
	}

	intro := pickRecommendationPhrase([]string{
		fmt.Sprintf("Si queres quedarte por menos de %.0f pesos, hoy hay algunas opciones que entran bien en ese presupuesto.", maxBudget),
		fmt.Sprintf("Si tu tope es de %.0f pesos, estas son las alternativas que hoy veo mas acomodadas a ese presupuesto.", maxBudget),
		fmt.Sprintf("Si estas buscando algo por debajo de %.0f pesos, hay cursos que ya entran dentro de ese rango.", maxBudget),
	}, variant)

	parts := []string{intro}
	for index, item := range items {
		priceSentence := fmt.Sprintf("entra con %.0f %s", item.Price, strings.TrimSpace(item.Currency))
		switch index {
		case 0:
			parts = append(parts, fmt.Sprintf("%s %s y %s.", item.Title, priceSentence, recommendationReason(item)))
		case 1:
			parts = append(parts, fmt.Sprintf("Tambien tenes %s, que %s y %s.", item.Title, priceSentence, recommendationReason(item)))
		default:
			parts = append(parts, fmt.Sprintf("Y otra opcion dentro del presupuesto es %s: %s y %s.", item.Title, priceSentence, recommendationReason(item)))
		}
	}
	parts = append(parts, pickRecommendationPhrase([]string{
		"Si queres, te las ordeno de mas economico a mas completo.",
		"Si te sirve, te digo cual conviene mas segun lo que quieras aprender.",
		"Si queres, vemos cual te rinde mejor por precio y por contenido.",
	}, variant))

	return strings.Join(parts, " ")
}

func filterCatalogByBudget(question string, catalog []RecommendationCatalogItem) []RecommendationCatalogItem {
	maxBudget, ok := extractRecommendationMaxBudget(question)
	if !ok {
		return catalog
	}

	filtered := make([]RecommendationCatalogItem, 0, len(catalog))
	for _, item := range catalog {
		if item.Price <= maxBudget {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

func extractRecommendationMaxBudget(question string) (float64, bool) {
	normalized := normalizeRecommendationText(question)
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`menos de\s+([0-9][0-9\.,]*)`),
		regexp.MustCompile(`por menos de\s+([0-9][0-9\.,]*)`),
		regexp.MustCompile(`hasta\s+([0-9][0-9\.,]*)`),
		regexp.MustCompile(`maximo\s+([0-9][0-9\.,]*)`),
		regexp.MustCompile(`no mas de\s+([0-9][0-9\.,]*)`),
		regexp.MustCompile(`menor a\s+([0-9][0-9\.,]*)`),
	}

	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(normalized)
		if len(match) != 2 {
			continue
		}

		amount, ok := parseRecommendationBudgetAmount(match[1])
		if ok {
			return amount, true
		}
	}

	return 0, false
}

func parseRecommendationBudgetAmount(raw string) (float64, bool) {
	digitsOnly := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, raw)
	if digitsOnly == "" {
		return 0, false
	}

	amount, err := strconv.ParseFloat(digitsOnly, 64)
	if err != nil || amount <= 0 {
		return 0, false
	}

	return amount, true
}

func buildBudgetUnavailableRecommendation(maxBudget float64) string {
	return fmt.Sprintf(
		"Ahora mismo no veo cursos publicados que encajen de verdad con ese tema por menos de %.0f pesos. Si queres, te puedo mostrar los mas cercanos al presupuesto o buscar otra area.",
		maxBudget,
	)
}

func buildConversationalRecommendation(question string, items []RecommendationCatalogItem, variant int) string {
	if len(items) == 0 {
		return "Ahora mismo no veo nada claro para recomendarte, pero si queres me contas un poco mas y lo pensamos juntos."
	}

	parts := make([]string, 0, len(items)+2)
	parts = append(parts, buildRecommendationLead(question, items[0], variant))
	parts = append(parts, buildPrimaryRecommendation(items[0], variant))

	if len(items) > 1 {
		parts = append(parts, buildSecondaryRecommendation(items[1], variant))
	}
	if len(items) > 2 {
		parts = append(parts, buildTertiaryRecommendation(items[2], variant))
	}

	parts = append(parts, buildRecommendationFollowUp(items[0], variant))
	return strings.Join(parts, " ")
}

func buildPrimaryRecommendation(item RecommendationCatalogItem, variant int) string {
	options := []string{
		fmt.Sprintf("Yo arrancaria por %s porque %s%s.", item.Title, recommendationReason(item), recommendationLevelSentence(item.Level)),
		fmt.Sprintf("La opcion que mas sentido me hace de entrada es %s, sobre todo porque %s%s.", item.Title, recommendationReason(item), recommendationLevelSentence(item.Level)),
		fmt.Sprintf("Si tuviera que elegir una para empezar, me iria por %s porque %s%s.", item.Title, recommendationReason(item), recommendationLevelSentence(item.Level)),
	}

	return pickRecommendationPhrase(options, variant)
}

func buildSecondaryRecommendation(item RecommendationCatalogItem, variant int) string {
	options := []string{
		fmt.Sprintf("Si queres una segunda opcion para mirar con calma, %s tambien te puede servir porque %s%s.", item.Title, recommendationReason(item), recommendationLevelSentence(item.Level)),
		fmt.Sprintf("Si preferis ir un poco mas de a poco, %s tambien encaja bien porque %s%s.", item.Title, recommendationReason(item), recommendationLevelSentence(item.Level)),
		fmt.Sprintf("Otra buena manera de entrar es %s, ya que %s%s.", item.Title, recommendationReason(item), recommendationLevelSentence(item.Level)),
	}

	return pickRecommendationPhrase(options, variant+1)
}

func buildTertiaryRecommendation(item RecommendationCatalogItem, variant int) string {
	options := []string{
		fmt.Sprintf("Y mas adelante, %s te puede sumar un monton porque %s%s.", item.Title, recommendationReason(item), recommendationLevelSentence(item.Level)),
		fmt.Sprintf("Si despues queres complementar lo anterior, %s tambien entra bien porque %s%s.", item.Title, recommendationReason(item), recommendationLevelSentence(item.Level)),
		fmt.Sprintf("Y si mas adelante queres ampliar un poco mas, %s tiene bastante sentido porque %s%s.", item.Title, recommendationReason(item), recommendationLevelSentence(item.Level)),
	}

	return pickRecommendationPhrase(options, variant+2)
}

func buildRecommendationLead(question string, item RecommendationCatalogItem, variant int) string {
	normalizedQuestion := normalizeRecommendationText(question)

	switch {
	case strings.Contains(normalizedQuestion, "pagina") || strings.Contains(normalizedQuestion, "web"):
		return pickRecommendationPhrase([]string{
			"Si queres meterte en paginas web, hay varias rutas lindas para empezar.",
			"Si tu idea va por el lado web, hay opciones que encajan muy bien.",
			"Para arrancar con paginas web, hay un recorrido bastante claro.",
		}, variant)
	case strings.Contains(normalizedQuestion, "backend"):
		return pickRecommendationPhrase([]string{
			"Si queres ir por backend, hay un par de caminos que te pueden servir mucho.",
			"Para meterte en backend, hay opciones bien armadas para arrancar.",
			"Si tu idea va por backend, hay una base linda para empezar sin marearte.",
		}, variant)
	case strings.Contains(normalizedQuestion, "frontend"):
		return pickRecommendationPhrase([]string{
			"Si queres arrancar con frontend, hay varias opciones copadas para entrar.",
			"Si tu idea va por frontend, hay cursos que te pueden acomodar bastante el inicio.",
			"Para meterte en frontend, hay un camino bastante natural para seguir.",
		}, variant)
	default:
		return pickRecommendationPhrase([]string{
			"Por lo que estas buscando, hay opciones que cierran bastante bien.",
			"Con esa idea que tenes en mente, hay un par de cursos que te pueden encajar.",
			"Si vas por ese lado, hay una forma bastante natural de arrancar.",
		}, variant)
	}
}

func buildRecommendationFollowUp(item RecommendationCatalogItem, variant int) string {
	switch normalizeRecommendationText(item.Category) {
	case "frontend":
		return pickRecommendationPhrase([]string{
			"Si queres, te digo por cual arrancaria yo segun si te tira mas lo visual o la parte de codigo. Que te interesa mas?",
			"Si te sirve, lo afinamos un poco segun si queres empezar por maquetado, por diseno o por interactividad. Por donde te ves arrancando?",
			"Si queres, vemos cual te conviene mas segun si te gusta la parte visual o hacer cosas con movimiento e interaccion. Que te llama mas?",
		}, variant)
	case "backend":
		return pickRecommendationPhrase([]string{
			"Si queres, lo bajamos un poco mas y vemos con cual te conviene empezar segun si buscas bases o algo mas practico. Que te serviria mas ahora?",
			"Si te sirve, te ayudo a elegir el mejor punto de entrada segun que tan de cero quieras arrancar. Queres algo bien base o mas aplicado?",
			"Si queres, vemos cual te conviene mas segun si te interesa entender fundamentos o mandarte a construir APIs rapido. Para donde te inclinarias?",
		}, variant)
	default:
		return pickRecommendationPhrase([]string{
			"Si queres, te ayudo a elegir con cual arrancar primero. Cual te llama mas?",
			"Si te sirve, lo afinamos un poco mas y vemos cual te conviene mas para empezar. Queres que te lo ordene por prioridad?",
			"Si queres, lo bajamos a algo mas concreto y te digo por donde arrancaria yo. Te interesa que lo pensemos juntos?",
		}, variant)
	}
}

func recommendationReason(item RecommendationCatalogItem) string {
	if description := recommendationDescriptionClause(item.ShortDescription); description != "" {
		return description
	}

	category := strings.TrimSpace(item.Category)
	if category == "" {
		return "encaja bastante con lo que me estas contando"
	}

	return "va por " + category
}

func recommendationDescriptionClause(description string) string {
	trimmed := strings.TrimSpace(strings.TrimSuffix(description, "."))
	if trimmed == "" {
		return ""
	}

	lowered := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(lowered, "aprende a "):
		return "te ensena a " + strings.TrimSpace(trimmed[len("aprende a "):])
	case strings.HasPrefix(lowered, "aprende "):
		return "te ayuda a entender " + strings.TrimSpace(trimmed[len("aprende "):])
	case strings.HasPrefix(lowered, "curso orientado a "):
		return "esta pensado para " + strings.TrimSpace(trimmed[len("curso orientado a "):])
	case strings.HasPrefix(lowered, "fundamentos de "):
		return "te da una base de " + strings.TrimSpace(trimmed[len("fundamentos de "):])
	default:
		return lowerFirst(trimmed)
	}
}

func pickRecommendationPhrase(options []string, variant int) string {
	if len(options) == 0 {
		return ""
	}
	if variant < 0 {
		variant = -variant
	}

	return options[variant%len(options)]
}

func recommendationStyleVariant(question, title string) int {
	sum := 0
	for _, value := range []string{normalizeRecommendationText(question), normalizeRecommendationText(title)} {
		for _, r := range value {
			sum += int(r)
		}
	}

	return sum
}

func lowerFirst(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 {
		return ""
	}

	runes[0] = []rune(strings.ToLower(string(runes[0])))[0]
	return string(runes)
}

func upperFirst(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 {
		return ""
	}

	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}

func recommendationLevelSentence(level string) string {
	trimmed := strings.TrimSpace(level)
	normalized := normalizeRecommendationText(trimmed)
	if trimmed == "" || normalized == "no especificado" {
		return ""
	}

	return ", con un nivel " + trimmed
}

func rankCatalog(question string, catalog []RecommendationCatalogItem) []rankedRecommendation {
	questionTokens := tokenizeRecommendationText(question)
	ranked := make([]rankedRecommendation, 0, len(catalog))

	for _, item := range catalog {
		titleText := normalizeRecommendationText(item.Title)
		categoryText := normalizeRecommendationText(item.Category)
		descriptionText := normalizeRecommendationText(item.ShortDescription)
		combinedText := strings.Join([]string{titleText, categoryText, descriptionText}, " ")

		score := 0
		for _, token := range questionTokens {
			for _, variant := range recommendationTokenVariants(token) {
				if strings.Contains(titleText, variant) {
					score += 5
				}
				if strings.Contains(categoryText, variant) {
					score += 4
				}
				if strings.Contains(descriptionText, variant) {
					score += 3
				}
				if strings.Contains(combinedText, variant) {
					score++
				}
			}
		}

		ranked = append(ranked, rankedRecommendation{
			item:  item,
			score: score,
		})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].item.Title < ranked[j].item.Title
		}
		return ranked[i].score > ranked[j].score
	})

	return ranked
}

func filterRankedByPrimaryDomain(question string, ranked []rankedRecommendation) []rankedRecommendation {
	domain := primaryRecommendationDomain(question)
	if domain == "" {
		return ranked
	}

	filtered := make([]rankedRecommendation, 0, len(ranked))
	for _, candidate := range ranked {
		if recommendationItemMatchesDomain(candidate.item, domain) {
			filtered = append(filtered, candidate)
		}
	}

	if len(filtered) == 0 {
		return ranked
	}

	return filtered
}

func primaryRecommendationDomain(question string) string {
	normalized := normalizeRecommendationText(question)

	switch {
	case strings.Contains(normalized, "abog"), strings.Contains(normalized, "derecho"), strings.Contains(normalized, "jurid"), strings.Contains(normalized, "contrato"), strings.Contains(normalized, "laboral"):
		return "derecho"
	case strings.Contains(normalized, "finanza"), strings.Contains(normalized, "ahorro"), strings.Contains(normalized, "presupuesto"), strings.Contains(normalized, "inversion"), strings.Contains(normalized, "rentabilidad"):
		return "finanzas"
	case strings.Contains(normalized, "backend"), strings.Contains(normalized, "frontend"), strings.Contains(normalized, "program"), strings.Contains(normalized, "software"), strings.Contains(normalized, "web"), strings.Contains(normalized, "pagina"), strings.Contains(normalized, "datos"), strings.Contains(normalized, "python"):
		return "tecnologia"
	case strings.Contains(normalized, "marketing"), strings.Contains(normalized, "redes"), strings.Contains(normalized, "ventas"), strings.Contains(normalized, "contenido"):
		return "marketing"
	default:
		return ""
	}
}

func recommendationItemMatchesDomain(item RecommendationCatalogItem, domain string) bool {
	category := normalizeRecommendationText(item.Category)
	title := normalizeRecommendationText(item.Title)
	description := normalizeRecommendationText(item.ShortDescription)
	combined := strings.Join([]string{category, title, description}, " ")

	switch domain {
	case "derecho":
		return strings.Contains(combined, "derecho") || strings.Contains(combined, "legal") || strings.Contains(combined, "laboral") || strings.Contains(combined, "contrato")
	case "finanzas":
		return strings.Contains(combined, "finanza") || strings.Contains(combined, "presupuesto") || strings.Contains(combined, "inversion") || strings.Contains(combined, "rentabilidad")
	case "tecnologia":
		return strings.Contains(combined, "backend") || strings.Contains(combined, "frontend") || strings.Contains(combined, "program") || strings.Contains(combined, "web") || strings.Contains(combined, "software") || strings.Contains(combined, "python") || strings.Contains(combined, "data")
	case "marketing":
		return strings.Contains(combined, "marketing") || strings.Contains(combined, "redes") || strings.Contains(combined, "campa") || strings.Contains(combined, "contenido")
	default:
		return false
	}
}

func tokenizeRecommendationText(value string) []string {
	normalized := normalizeRecommendationText(value)
	fields := strings.FieldsFunc(normalized, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	stopwords := map[string]struct{}{
		"a": {}, "al": {}, "algo": {}, "aprende": {}, "aprender": {}, "con": {}, "como": {},
		"de": {}, "del": {}, "el": {}, "en": {}, "es": {}, "la": {}, "las": {}, "lo": {},
		"los": {}, "me": {}, "mi": {}, "mis": {}, "necesito": {}, "para": {}, "por": {},
		"porque": {}, "quiero": {}, "que": {}, "una": {}, "uno": {}, "unos": {}, "un": {}, "y": {},
	}

	seen := make(map[string]struct{}, len(fields))
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if len(field) < 3 {
			continue
		}
		if _, blocked := stopwords[field]; blocked {
			continue
		}
		if _, exists := seen[field]; exists {
			continue
		}
		seen[field] = struct{}{}
		tokens = append(tokens, field)
	}

	return tokens
}

func recommendationTokenVariants(token string) []string {
	variants := []string{token}
	if len(token) >= 6 {
		variants = append(variants, token[:6])
	}
	if strings.HasSuffix(token, "ar") || strings.HasSuffix(token, "er") || strings.HasSuffix(token, "ir") {
		if len(token) > 4 {
			variants = append(variants, token[:len(token)-2])
		}
	}
	variants = append(variants, recommendationDomainVariants(token)...)

	seen := make(map[string]struct{}, len(variants))
	deduped := make([]string, 0, len(variants))
	for _, variant := range variants {
		if len(variant) < 3 {
			continue
		}
		if _, exists := seen[variant]; exists {
			continue
		}
		seen[variant] = struct{}{}
		deduped = append(deduped, variant)
	}

	return deduped
}

func recommendationDomainVariants(token string) []string {
	switch normalizeRecommendationText(token) {
	case "abogacia", "abogado", "abogados", "derecho", "legal", "juridico", "juridica":
		return []string{"derecho", "legal", "juridic", "laboral", "contrato", "contratos"}
	case "contrato", "contratos", "laboral":
		return []string{"derecho", "legal", "laboral", "contrato", "contratos"}
	case "finanza", "finanzas", "ahorro", "inversion", "inversiones", "presupuesto", "rentabilidad":
		return []string{"finanzas", "presupuesto", "rentabilidad", "caja", "costos", "emprendedor"}
	case "marketing", "redes", "publicidad", "contenido":
		return []string{"marketing", "redes", "campanas", "clientes", "digital"}
	case "datos", "data", "analytics", "analisis", "power", "python":
		return []string{"data", "datos", "analytics", "power bi", "python"}
	case "web", "frontend", "backend", "programacion", "codigo", "software":
		return []string{"frontend", "backend", "programacion", "web", "software"}
	default:
		return nil
	}
}

func shouldAskClarifyingQuestion(question string, tokens []string) bool {
	normalized := normalizeRecommendationText(question)
	if len(tokens) == 0 {
		return true
	}

	greetings := map[string]struct{}{
		"hola": {}, "holi": {}, "buenas": {}, "buen dia": {}, "buenas tardes": {},
		"buenas noches": {}, "hello": {}, "hi": {}, "hey": {},
	}
	if _, ok := greetings[normalized]; ok {
		return true
	}

	return len(tokens) == 1 && normalized == tokens[0] && len(tokens[0]) <= 4
}

func hasSpecificTopicTokens(tokens []string) bool {
	genericTokens := map[string]struct{}{
		"barato": {}, "baratos": {}, "caro": {}, "caros": {}, "curso": {}, "cursos": {},
		"economico": {}, "economicos": {}, "menos": {}, "peso": {}, "pesos": {}, "precio": {},
		"precios": {}, "sale": {}, "salen": {}, "tenes": {}, "tienes": {}, "tope": {},
		"busco": {}, "buscando": {}, "estoy": {}, "quiero": {}, "opcion": {}, "opciones": {},
		"estudiar": {}, "estudio": {}, "trabajo": {}, "laburo": {}, "hobby": {}, "objetivo": {},
		"meta": {}, "desde": {}, "cero": {}, "empezando": {}, "empezar": {}, "empiezo": {},
		"principiante": {}, "basico": {}, "basica": {}, "intermedio": {}, "avanzado": {},
		"experiencia": {}, "completo": {}, "completa": {}, "corto": {}, "corta": {},
		"intensivo": {}, "practico": {}, "practica": {}, "simple": {}, "relacionado": {},
		"relacionada": {}, "gustaria": {}, "mejorar": {}, "primer": {}, "primero": {},
		"interesa": {}, "interesan": {}, "gusta": {}, "gustan": {}, "dame": {}, "otra": {},
		"otro": {}, "segunda": {}, "segundo": {}, "tercera": {}, "tercero": {}, "ultima": {},
	}

	for _, token := range tokens {
		if _, generic := genericTokens[token]; generic {
			continue
		}
		if _, err := strconv.Atoi(token); err == nil {
			continue
		}
		return true
	}

	return false
}

func buildClarifyingRecommendation(catalog []RecommendationCatalogItem) string {
	categories := make([]string, 0, 4)
	seen := make(map[string]struct{})
	for _, item := range catalog {
		category := strings.TrimSpace(item.Category)
		if category == "" {
			continue
		}
		if _, exists := seen[category]; exists {
			continue
		}
		seen[category] = struct{}{}
		categories = append(categories, category)
		if len(categories) == 4 {
			break
		}
	}

	if len(categories) == 0 {
		return "Hola! Te doy una mano con eso :) Contame que te gustaria aprender y te oriento con lo que haya publicado."
	}

	return fmt.Sprintf(
		"Hola! Te doy una mano con eso :) Hoy tenemos opciones de %s. Contame un poco mas que te interesa y te sugiero algo mucho mas afinado.",
		joinRecommendationCategories(categories),
	)
}

func joinRecommendationCategories(categories []string) string {
	switch len(categories) {
	case 0:
		return ""
	case 1:
		return categories[0]
	case 2:
		return categories[0] + " y " + categories[1]
	default:
		return strings.Join(categories[:len(categories)-1], ", ") + " y " + categories[len(categories)-1]
	}
}

func normalizeRecommendationText(value string) string {
	replacer := strings.NewReplacer(
		"\u00e1", "a", "\u00e9", "e", "\u00ed", "i", "\u00f3", "o", "\u00fa", "u",
		"\u00c1", "a", "\u00c9", "e", "\u00cd", "i", "\u00d3", "o", "\u00da", "u",
		"\u00f1", "n", "\u00d1", "n",
	)

	return strings.ToLower(strings.TrimSpace(replacer.Replace(value)))
}
