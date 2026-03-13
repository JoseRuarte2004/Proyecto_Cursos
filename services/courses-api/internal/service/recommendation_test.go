package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeRecommendationAdvisor struct {
	recommendFn func(ctx context.Context, systemPrompt string, history []RecommendationMessage, question string) (string, error)
}

func (f fakeRecommendationAdvisor) Recommend(ctx context.Context, systemPrompt string, history []RecommendationMessage, question string) (string, error) {
	return f.recommendFn(ctx, systemPrompt, history, question)
}

type fakeRecommendationAvailabilityLookup struct {
	getAvailabilityFn func(ctx context.Context, courseID string) (*RecommendationAvailability, error)
}

func (f fakeRecommendationAvailabilityLookup) GetAvailability(ctx context.Context, courseID string) (*RecommendationAvailability, error) {
	return f.getAvailabilityFn(ctx, courseID)
}

func TestRecommendationServiceReturnsAdvisorAnswer(t *testing.T) {
	t.Parallel()

	expectedCatalog := []RecommendationCatalogItem{
		{
			ID:               "course-1",
			Title:            "Go Desde Cero",
			ShortDescription: "Aprende fundamentos de programacion en Go.",
			Level:            "no especificado",
			Category:         "backend",
			Price:            18000,
			Currency:         "ARS",
		},
	}

	var capturedPrompt string
	var capturedHistory []RecommendationMessage
	var capturedQuestion string

	svc := NewRecommendationService(
		&mockCourseRepository{
			listRecommendationCatalogFn: func(context.Context) ([]RecommendationCatalogItem, error) {
				return expectedCatalog, nil
			},
		},
		nil,
		nil,
		nil,
		fakeRecommendationAdvisor{
			recommendFn: func(_ context.Context, systemPrompt string, history []RecommendationMessage, question string) (string, error) {
				capturedPrompt = systemPrompt
				capturedHistory = history
				capturedQuestion = question
				return "Te recomiendo Go Desde Cero.", nil
			},
		},
	)

	result, err := svc.Recommend(context.Background(), "Jose", "Quiero aprender a programar", []RecommendationMessage{
		{Role: "user", Content: "Busco algo practico"},
		{Role: "assistant", Content: "Te puedo orientar segun el catalogo."},
	})

	require.NoError(t, err)
	require.Equal(t, "Quiero aprender a programar", capturedQuestion)
	require.Equal(t, []RecommendationMessage{
		{Role: "user", Content: "Busco algo practico"},
		{Role: "assistant", Content: "Te puedo orientar segun el catalogo."},
	}, capturedHistory)
	require.Contains(t, capturedPrompt, "\"title\": \"Go Desde Cero\"")
	require.Contains(t, capturedPrompt, "\"price\": 18000")
	require.Contains(t, capturedPrompt, "secretaria academica inteligente")
	require.Contains(t, capturedPrompt, "Si aun no hay suficiente contexto, hace solo 1 o 2 preguntas utiles")
	require.Contains(t, capturedPrompt, "¡Hola, Jose! 👋")
	require.Equal(t, "Te recomiendo Go Desde Cero.", result.Answer)
	require.Equal(t, expectedCatalog, result.Catalog)
}

func TestRecommendationServiceRequiresQuestion(t *testing.T) {
	t.Parallel()

	svc := NewRecommendationService(
		&mockCourseRepository{
			listRecommendationCatalogFn: func(context.Context) ([]RecommendationCatalogItem, error) {
				return nil, nil
			},
		},
		nil,
		nil,
		nil,
		fakeRecommendationAdvisor{
			recommendFn: func(context.Context, string, []RecommendationMessage, string) (string, error) {
				return "", nil
			},
		},
	)

	result, err := svc.Recommend(context.Background(), "", "   ", nil)

	require.Nil(t, result)
	require.ErrorIs(t, err, ErrQuestionRequired)
}

func TestRecommendationServiceFallsBackWithoutAdvisorOnFirstTurn(t *testing.T) {
	t.Parallel()

	svc := NewRecommendationService(
		&mockCourseRepository{
			listRecommendationCatalogFn: func(context.Context) ([]RecommendationCatalogItem, error) {
				return []RecommendationCatalogItem{
					{
						ID:               "course-1",
						Title:            "Programacion Inicial",
						ShortDescription: "Curso para empezar en desarrollo.",
						Level:            "no especificado",
						Category:         "programacion",
						Price:            12000,
						Currency:         "ARS",
					},
				}, nil
			},
		},
		nil,
		nil,
		nil,
		nil,
	)

	result, err := svc.Recommend(context.Background(), "Jose", "Quiero empezar a programar", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result.Answer, "¡Hola, Jose! 👋")
	require.Contains(t, result.Answer, "Estoy aqui para que tu experiencia con la app de cursos sea mas simple y util.")
	require.Contains(t, result.Answer, "mundo tech")
	require.Contains(t, result.Answer, "Estas empezando desde cero o ya tenes algo de experiencia?")
	require.Contains(t, result.Answer, "Lo queres aprender por trabajo, hobby o estudio?")
}

func TestRecommendationServiceReturnsFriendlyMessageWithoutPublishedCatalog(t *testing.T) {
	t.Parallel()

	svc := NewRecommendationService(
		&mockCourseRepository{
			listRecommendationCatalogFn: func(context.Context) ([]RecommendationCatalogItem, error) {
				return []RecommendationCatalogItem{}, nil
			},
		},
		nil,
		nil,
		nil,
		fakeRecommendationAdvisor{
			recommendFn: func(context.Context, string, []RecommendationMessage, string) (string, error) {
				t.Fatal("advisor should not run without catalog")
				return "", nil
			},
		},
	)

	result, err := svc.Recommend(context.Background(), "", "Necesito una recomendacion", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result.Answer, "justo ahora no veo cursos publicados")
	require.Nil(t, result.Catalog)
}

func TestRecommendationServiceFallsBackWhenAdvisorFails(t *testing.T) {
	t.Parallel()

	svc := NewRecommendationService(
		&mockCourseRepository{
			listRecommendationCatalogFn: func(context.Context) ([]RecommendationCatalogItem, error) {
				return []RecommendationCatalogItem{
					{
						ID:               "course-1",
						Title:            "Backend Desde Cero",
						ShortDescription: "Aprende fundamentos de backend.",
						Level:            "no especificado",
						Category:         "backend",
						Price:            18000,
						Currency:         "ARS",
					},
					{
						ID:               "course-2",
						Title:            "Frontend Practico",
						ShortDescription: "Interfaces y componentes.",
						Level:            "no especificado",
						Category:         "frontend",
						Price:            14000,
						Currency:         "ARS",
					},
				}, nil
			},
		},
		nil,
		nil,
		nil,
		fakeRecommendationAdvisor{
			recommendFn: func(context.Context, string, []RecommendationMessage, string) (string, error) {
				return "", errors.New("quota exceeded")
			},
		},
	)

	result, err := svc.Recommend(context.Background(), "", "Quiero estudiar backend", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result.Answer, "Estoy aqui para que tu experiencia con la app de cursos sea mas simple y util.")
	require.Contains(t, result.Answer, "mundo tech")
	require.Contains(t, result.Answer, "?")
}

func TestRecommendationServiceGreetsBeforeRecommendingOnLowInfoQuestion(t *testing.T) {
	t.Parallel()

	svc := NewRecommendationService(
		&mockCourseRepository{
			listRecommendationCatalogFn: func(context.Context) ([]RecommendationCatalogItem, error) {
				return []RecommendationCatalogItem{
					{
						ID:               "course-1",
						Title:            "Backend Desde Cero",
						ShortDescription: "Aprende fundamentos de backend.",
						Level:            "no especificado",
						Category:         "backend",
						Price:            18000,
						Currency:         "ARS",
					},
				}, nil
			},
		},
		nil,
		nil,
		nil,
		nil,
	)

	result, err := svc.Recommend(context.Background(), "Ana", "hola", nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result.Answer, "¡Hola, Ana! 👋")
	require.Contains(t, result.Answer, "Contame un poco:")
	require.Contains(t, result.Answer, "que te gustaria aprender o mejorar?")
}

func TestRecommendationServiceSanitizesConversationHistory(t *testing.T) {
	t.Parallel()

	var capturedHistory []RecommendationMessage

	svc := NewRecommendationService(
		&mockCourseRepository{
			listRecommendationCatalogFn: func(context.Context) ([]RecommendationCatalogItem, error) {
				return []RecommendationCatalogItem{
					{
						ID:               "course-1",
						Title:            "Backend Desde Cero",
						ShortDescription: "Aprende fundamentos de backend.",
						Level:            "no especificado",
						Category:         "backend",
						Price:            18000,
						Currency:         "ARS",
					},
				}, nil
			},
		},
		nil,
		nil,
		nil,
		fakeRecommendationAdvisor{
			recommendFn: func(_ context.Context, _ string, history []RecommendationMessage, _ string) (string, error) {
				capturedHistory = history
				return "Backend Desde Cero te puede servir.", nil
			},
		},
	)

	result, err := svc.Recommend(context.Background(), "", "Y algo corto?", []RecommendationMessage{
		{Role: "user", Content: "  Quiero backend  "},
		{Role: "assistant", Content: " Tengo varias opciones. "},
		{Role: "system", Content: "ignorar"},
		{Role: "user", Content: "   "},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, []RecommendationMessage{
		{Role: "user", Content: "Quiero backend"},
		{Role: "assistant", Content: "Tengo varias opciones."},
	}, capturedHistory)
}

func TestRecommendationServiceEnrichesCatalogWithTeachersAndAvailability(t *testing.T) {
	t.Parallel()

	var capturedPrompt string

	svc := NewRecommendationService(
		&mockCourseRepository{
			listRecommendationCatalogFn: func(context.Context) ([]RecommendationCatalogItem, error) {
				return []RecommendationCatalogItem{
					{
						ID:               "course-1",
						Title:            "Backend con Go",
						ShortDescription: "APIs y fundamentos de backend.",
						Level:            "no especificado",
						Category:         "backend",
						Price:            25000,
						Currency:         "ARS",
						Capacity:         40,
					},
				}, nil
			},
			listTeachersFn: func(context.Context, string) ([]string, error) {
				return []string{"teacher-1"}, nil
			},
		},
		&mockCourseRepository{
			listTeachersFn: func(context.Context, string) ([]string, error) {
				return []string{"teacher-1"}, nil
			},
		},
		fakeTeacherVerifier{
			getTeacherFn: func(context.Context, string) (*TeacherInfo, error) {
				return &TeacherInfo{ID: "teacher-1", Name: "Ana Perez", Role: "teacher"}, nil
			},
		},
		fakeRecommendationAvailabilityLookup{
			getAvailabilityFn: func(context.Context, string) (*RecommendationAvailability, error) {
				return &RecommendationAvailability{
					CourseID:  "course-1",
					Capacity:  40,
					Available: 12,
				}, nil
			},
		},
		fakeRecommendationAdvisor{
			recommendFn: func(_ context.Context, systemPrompt string, _ []RecommendationMessage, _ string) (string, error) {
				capturedPrompt = systemPrompt
				return "Backend con Go te puede servir.", nil
			},
		},
	)

	result, err := svc.Recommend(context.Background(), "", "Quiero estudiar backend", []RecommendationMessage{
		{Role: "user", Content: "Quiero estudiar backend"},
		{Role: "assistant", Content: "Contame si es para trabajo o para arrancar desde cero."},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Catalog, 1)
	require.Equal(t, []string{"Ana Perez"}, result.Catalog[0].Teachers)
	require.Equal(t, 12, result.Catalog[0].AvailableSeats)
	require.Contains(t, capturedPrompt, "\"teachers\": [")
	require.Contains(t, capturedPrompt, "Ana Perez")
	require.Contains(t, capturedPrompt, "\"availableSeats\": 12")
}

func TestRecommendationServiceFiltersAdvisorContextWhenUserChangesTopic(t *testing.T) {
	t.Parallel()

	var capturedPrompt string
	var capturedHistory []RecommendationMessage

	svc := NewRecommendationService(
		&mockCourseRepository{
			listRecommendationCatalogFn: func(context.Context) ([]RecommendationCatalogItem, error) {
				return []RecommendationCatalogItem{
					{
						ID:               "course-1",
						Title:            "Backend con Go",
						ShortDescription: "APIs y fundamentos de backend para empezar a construir servicios reales.",
						Category:         "backend",
						Price:            25000,
						Currency:         "ARS",
					},
					{
						ID:               "course-2",
						Title:            "Finanzas Personales y Presupuesto",
						ShortDescription: "Ordena gastos, arma un presupuesto y toma mejores decisiones con tu dinero.",
						Category:         "finanzas",
						Price:            18000,
						Currency:         "ARS",
					},
					{
						ID:               "course-3",
						Title:            "Analisis Financiero para Emprendedores",
						ShortDescription: "Aprende a leer numeros clave, costos y rentabilidad para decidir con mas claridad.",
						Category:         "finanzas",
						Price:            22000,
						Currency:         "ARS",
					},
				}, nil
			},
		},
		nil,
		nil,
		nil,
		fakeRecommendationAdvisor{
			recommendFn: func(_ context.Context, systemPrompt string, history []RecommendationMessage, _ string) (string, error) {
				capturedPrompt = systemPrompt
				capturedHistory = history
				return "Te puedo recomendar algo de finanzas.", nil
			},
		},
	)

	result, err := svc.Recommend(context.Background(), "Jose", "Me confundi, en realidad quiero estudiar algo de finanzas.", []RecommendationMessage{
		{Role: "user", Content: "Quiero estudiar programacion"},
		{Role: "assistant", Content: "Buenisimo. Decime si estas empezando desde cero o ya viste algo."},
		{Role: "user", Content: "Lo quiero para trabajo y estoy empezando desde cero."},
		{Role: "assistant", Content: "Perfecto, con eso ya te puedo orientar mejor."},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "Te puedo recomendar algo de finanzas.", result.Answer)
	require.Equal(t, []RecommendationMessage{
		{Role: "user", Content: "Lo quiero para trabajo y estoy empezando desde cero."},
		{Role: "assistant", Content: "Perfecto, con eso ya te puedo orientar mejor."},
	}, capturedHistory)
	require.Contains(t, capturedPrompt, "\"title\": \"Finanzas Personales y Presupuesto\"")
	require.Contains(t, capturedPrompt, "\"title\": \"Analisis Financiero para Emprendedores\"")
	require.NotContains(t, capturedPrompt, "\"title\": \"Backend con Go\"")
	require.Contains(t, capturedPrompt, "Consulta actual prioritaria del usuario:")
	require.Contains(t, capturedPrompt, "Me confundi, en realidad quiero estudiar algo de finanzas.")
}

func TestRecommendationServiceExcludesRejectedPreviousOptionFromAdvisorCatalog(t *testing.T) {
	t.Parallel()

	var capturedPrompt string

	svc := NewRecommendationService(
		&mockCourseRepository{
			listRecommendationCatalogFn: func(context.Context) ([]RecommendationCatalogItem, error) {
				return []RecommendationCatalogItem{
					{
						ID:               "course-1",
						Title:            "Backend con Go",
						ShortDescription: "APIs y fundamentos de backend para empezar a construir servicios reales.",
						Category:         "backend",
						Price:            25000,
						Currency:         "ARS",
					},
					{
						ID:               "course-2",
						Title:            "Python para Analisis de Datos",
						ShortDescription: "Aprende automatizacion y manejo de datos con Python.",
						Category:         "tecnologia",
						Price:            23000,
						Currency:         "ARS",
					},
					{
						ID:               "course-3",
						Title:            "Data Analytics con Power BI",
						ShortDescription: "Modela datos y crea dashboards para tomar decisiones mejor.",
						Category:         "data",
						Price:            24000,
						Currency:         "ARS",
					},
				}, nil
			},
		},
		nil,
		nil,
		nil,
		fakeRecommendationAdvisor{
			recommendFn: func(_ context.Context, systemPrompt string, _ []RecommendationMessage, _ string) (string, error) {
				capturedPrompt = systemPrompt
				return "Te muestro otras opciones.", nil
			},
		},
	)

	result, err := svc.Recommend(context.Background(), "Jose", "No me interesa lo primero, dame otra opcion.", []RecommendationMessage{
		{Role: "user", Content: "Quiero estudiar programacion para trabajo."},
		{Role: "assistant", Content: "Yo arrancaria por Backend con Go. Si queres, tambien te puedo mostrar Python para Analisis de Datos y Data Analytics con Power BI."},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "Te muestro otras opciones.", result.Answer)
	require.NotContains(t, capturedPrompt, "\"title\": \"Backend con Go\"")
	require.Contains(t, capturedPrompt, "\"title\": \"Python para Analisis de Datos\"")
	require.Contains(t, capturedPrompt, "\"title\": \"Data Analytics con Power BI\"")
}

func TestBuildFallbackRecommendationGreetsAndAsksContextOnFirstTurn(t *testing.T) {
	t.Parallel()

	answer := BuildFallbackRecommendation("Quiero aprender a hacer paginas", []RecommendationCatalogItem{
		{
			ID:               "course-1",
			Title:            "Desarrollo Web Desde Cero",
			ShortDescription: "Aprende a crear paginas web modernas desde cero con enfoque practico en estructura, estilos y publicacion.",
			Level:            "no especificado",
			Category:         "frontend",
			Price:            15000,
			Currency:         "ARS",
		},
	}, RecommendationResponseOptions{UserName: "Jose"})

	require.Contains(t, answer, "¡Hola, Jose! 👋")
	require.Contains(t, answer, "mundo tech")
	require.Contains(t, answer, "Estas empezando desde cero o ya tenes algo de experiencia?")
	require.Contains(t, answer, "Lo queres aprender por trabajo, hobby o estudio?")
}

func TestBuildFallbackRecommendationRecommendsAfterContext(t *testing.T) {
	t.Parallel()

	answer := BuildFallbackRecommendation("Lo quiero aprender para trabajo y estoy empezando desde cero.", []RecommendationCatalogItem{
		{
			ID:               "course-1",
			Title:            "Desarrollo Web Desde Cero",
			ShortDescription: "Aprende a crear paginas web modernas desde cero con enfoque practico en estructura, estilos y publicacion.",
			Level:            "no especificado",
			Category:         "frontend",
			Price:            15000,
			Currency:         "ARS",
		},
		{
			ID:               "course-2",
			Title:            "HTML y CSS Practico",
			ShortDescription: "Curso orientado a maquetar paginas, trabajar con layouts responsivos y mejorar la presentacion visual de sitios.",
			Level:            "no especificado",
			Category:         "frontend",
			Price:            12000,
			Currency:         "ARS",
		},
		{
			ID:               "course-3",
			Title:            "JavaScript Inicial",
			ShortDescription: "Fundamentos de JavaScript para agregar interactividad a paginas web y dar el salto a desarrollo frontend.",
			Level:            "no especificado",
			Category:         "frontend",
			Price:            18000,
			Currency:         "ARS",
		},
	}, RecommendationResponseOptions{
		History: []RecommendationMessage{
			{Role: "user", Content: "Quiero aprender a hacer paginas"},
			{Role: "assistant", Content: "Decime si estas empezando desde cero o ya viste algo."},
		},
	})

	require.Contains(t, answer, "estas opciones creo que te pueden servir")
	require.Contains(t, answer, "Desarrollo Web Desde Cero")
	require.Contains(t, answer, "HTML y CSS Practico")
	require.Contains(t, answer, "JavaScript Inicial")
	require.Contains(t, answer, "Si queres")
	require.NotContains(t, answer, "Nivel:")
	require.NotContains(t, answer, "no especificado")
}

func TestBuildFallbackRecommendationPrioritizesLawCoursesForLawQueries(t *testing.T) {
	t.Parallel()

	answer := BuildFallbackRecommendation("Estoy empezando desde cero y lo quiero para trabajo.", []RecommendationCatalogItem{
		{
			ID:               "course-1",
			Title:            "HTML y CSS Practico",
			ShortDescription: "Curso orientado a maquetar paginas, trabajar con layouts responsivos y mejorar la presentacion visual de sitios.",
			Level:            "no especificado",
			Category:         "frontend",
			Price:            12000,
			Currency:         "ARS",
		},
		{
			ID:               "course-2",
			Title:            "Introduccion al Derecho Laboral",
			ShortDescription: "Entiende derechos y obligaciones en relaciones laborales y tipos de contrato.",
			Level:            "no especificado",
			Category:         "derecho",
			Price:            21000,
			Currency:         "ARS",
		},
		{
			ID:               "course-3",
			Title:            "Contratos Comerciales para Pymes",
			ShortDescription: "Revisa clausulas clave y buenas practicas para leer, redactar y negociar contratos comerciales.",
			Level:            "no especificado",
			Category:         "derecho",
			Price:            26000,
			Currency:         "ARS",
		},
	}, RecommendationResponseOptions{
		History: []RecommendationMessage{
			{Role: "user", Content: "Quiero algo de abogacia"},
			{Role: "assistant", Content: "Decime si estas empezando desde cero o ya tenes experiencia."},
		},
	})

	require.Contains(t, answer, "Introduccion al Derecho Laboral")
	require.Contains(t, answer, "Contratos Comerciales para Pymes")
	require.NotContains(t, answer, "HTML y CSS Practico")
}

func TestBuildFallbackRecommendationRespectsMaximumBudget(t *testing.T) {
	t.Parallel()

	answer := BuildFallbackRecommendation("Que cursos tenes por menos de 20000 pesos?", []RecommendationCatalogItem{
		{
			ID:               "course-1",
			Title:            "Data Analytics con Power BI",
			ShortDescription: "Aprende a modelar datos y crear dashboards.",
			Level:            "no especificado",
			Category:         "data",
			Price:            25000,
			Currency:         "ARS",
		},
		{
			ID:               "course-2",
			Title:            "Excel Aplicado a Gestion",
			ShortDescription: "Domina formulas, tablas dinamicas y reportes operativos.",
			Level:            "no especificado",
			Category:         "herramientas",
			Price:            14000,
			Currency:         "ARS",
		},
		{
			ID:               "course-3",
			Title:            "Marketing Digital y Redes Sociales",
			ShortDescription: "Disena campanas y contenidos para mejorar tu presencia digital.",
			Level:            "no especificado",
			Category:         "marketing",
			Price:            19000,
			Currency:         "ARS",
		},
	}, RecommendationResponseOptions{
		History: []RecommendationMessage{
			{Role: "user", Content: "Estoy buscando algo economico"},
			{Role: "assistant", Content: "Perfecto, tambien puedo ordenar opciones por presupuesto."},
		},
	})

	require.Contains(t, answer, "Excel Aplicado a Gestion")
	require.Contains(t, answer, "Marketing Digital y Redes Sociales")
	require.NotContains(t, answer, "Data Analytics con Power BI")
}

func TestBuildFallbackRecommendationAnswersTeacherQuestion(t *testing.T) {
	t.Parallel()

	answer := BuildFallbackRecommendation("Que profesores se encuentran dando clases?", []RecommendationCatalogItem{
		{
			ID:               "course-1",
			Title:            "Backend con Go",
			ShortDescription: "APIs y fundamentos de backend.",
			Category:         "backend",
			Teachers:         []string{"Ana Perez"},
		},
		{
			ID:               "course-2",
			Title:            "Finanzas Personales y Presupuesto",
			ShortDescription: "Ordena gastos y arma un presupuesto.",
			Category:         "finanzas",
			Teachers:         []string{"Marcos Diaz"},
		},
	}, RecommendationResponseOptions{})

	require.Contains(t, answer, "Hoy estos cursos ya tienen profesores asignados")
	require.Contains(t, answer, "Backend con Go: Ana Perez")
	require.Contains(t, answer, "Finanzas Personales y Presupuesto: Marcos Diaz")
}

func TestBuildFallbackRecommendationAnswersSeatsQuestion(t *testing.T) {
	t.Parallel()

	answer := BuildFallbackRecommendation("Que cursos todavia tienen cupos?", []RecommendationCatalogItem{
		{
			ID:             "course-1",
			Title:          "Backend con Go",
			Category:       "backend",
			Capacity:       40,
			AvailableSeats: 12,
		},
		{
			ID:             "course-2",
			Title:          "Introduccion al Derecho Laboral",
			Category:       "derecho",
			Capacity:       30,
			AvailableSeats: 0,
		},
	}, RecommendationResponseOptions{})

	require.Contains(t, answer, "Te paso los cursos con cupos visibles")
	require.Contains(t, answer, "Backend con Go: quedan 12 cupos de 40")
	require.Contains(t, answer, "Introduccion al Derecho Laboral: ahora mismo no tiene cupos disponibles sobre 30 lugares")
}

func TestBuildFallbackRecommendationAnswersDurationQuestionWhenDurationMissing(t *testing.T) {
	t.Parallel()

	answer := BuildFallbackRecommendation("Cuanto duran los cursos?", []RecommendationCatalogItem{
		{
			ID:               "course-1",
			Title:            "Backend con Go",
			ShortDescription: "APIs y fundamentos de backend.",
			Category:         "backend",
		},
	}, RecommendationResponseOptions{})

	require.Contains(t, answer, "Todavia no tengo la duracion publicada")
	require.Contains(t, answer, "precio, profesores, cupos")
}

func TestBuildFallbackRecommendationDoesNotGetStuckOnPreviousMetadataIntent(t *testing.T) {
	t.Parallel()

	answer := BuildFallbackRecommendation("Quiero que me expliques de que se trata javascript", []RecommendationCatalogItem{
		{
			ID:               "course-1",
			Title:            "JavaScript Inicial",
			ShortDescription: "Fundamentos de JavaScript para agregar interactividad a paginas web y dar el salto a desarrollo frontend.",
			Category:         "frontend",
			Teachers:         []string{"Javier Ruarte"},
			Price:            18000,
			Currency:         "ARS",
			Capacity:         30,
			AvailableSeats:   9,
		},
	}, RecommendationResponseOptions{
		History: []RecommendationMessage{
			{Role: "user", Content: "Que profesores se encuentran dando clases?"},
			{Role: "assistant", Content: "Hoy estos cursos ya tienen profesores asignados."},
		},
	})

	require.Contains(t, answer, "JavaScript Inicial va por este lado")
	require.Contains(t, answer, "Hoy lo esta dando Javier Ruarte")
	require.NotContains(t, answer, "profesores asignados")
}

func TestBuildFallbackRecommendationPrioritizesLatestTopicWhenUserCorrectsThemselves(t *testing.T) {
	t.Parallel()

	answer := BuildFallbackRecommendation("Me confundi, en realidad quiero estudiar algo relacionado a finanzas.", []RecommendationCatalogItem{
		{
			ID:               "course-1",
			Title:            "Backend con Go",
			ShortDescription: "APIs y fundamentos de backend para empezar a construir servicios reales.",
			Category:         "backend",
			Price:            25000,
			Currency:         "ARS",
		},
		{
			ID:               "course-2",
			Title:            "Finanzas Personales y Presupuesto",
			ShortDescription: "Ordena gastos, arma un presupuesto y toma mejores decisiones con tu dinero.",
			Category:         "finanzas",
			Price:            18000,
			Currency:         "ARS",
		},
		{
			ID:               "course-3",
			Title:            "Analisis Financiero para Emprendedores",
			ShortDescription: "Aprende a leer numeros clave, costos y rentabilidad para decidir con mas claridad.",
			Category:         "finanzas",
			Price:            22000,
			Currency:         "ARS",
		},
	}, RecommendationResponseOptions{
		History: []RecommendationMessage{
			{Role: "user", Content: "Quiero estudiar programacion"},
			{Role: "assistant", Content: "Buenisimo. Decime si estas empezando desde cero o ya viste algo."},
			{Role: "user", Content: "Lo quiero para trabajo y estoy empezando desde cero."},
			{Role: "assistant", Content: "Perfecto, con eso ya te puedo orientar mejor."},
		},
	})

	require.Contains(t, answer, "Finanzas Personales y Presupuesto")
	require.Contains(t, answer, "Analisis Financiero para Emprendedores")
	require.NotContains(t, answer, "Backend con Go")
}

func TestBuildFallbackRecommendationSkipsRejectedFirstOption(t *testing.T) {
	t.Parallel()

	answer := BuildFallbackRecommendation("No me interesa lo primero, dame otra opcion.", []RecommendationCatalogItem{
		{
			ID:               "course-1",
			Title:            "Backend con Go",
			ShortDescription: "APIs y fundamentos de backend para empezar a construir servicios reales.",
			Category:         "backend",
			Price:            25000,
			Currency:         "ARS",
		},
		{
			ID:               "course-2",
			Title:            "Python para Analisis de Datos",
			ShortDescription: "Aprende automatizacion y manejo de datos con Python.",
			Category:         "tecnologia",
			Price:            23000,
			Currency:         "ARS",
		},
		{
			ID:               "course-3",
			Title:            "Data Analytics con Power BI",
			ShortDescription: "Modela datos y crea dashboards para tomar decisiones mejor.",
			Category:         "data",
			Price:            24000,
			Currency:         "ARS",
		},
	}, RecommendationResponseOptions{
		History: []RecommendationMessage{
			{Role: "user", Content: "Quiero estudiar programacion para trabajo y estoy empezando desde cero."},
			{Role: "assistant", Content: "Yo arrancaria por Backend con Go. Si queres, tambien te puedo mostrar Python para Analisis de Datos y Data Analytics con Power BI."},
		},
	})

	require.NotContains(t, answer, "Backend con Go")
	require.True(t,
		strings.Contains(answer, "Python para Analisis de Datos") || strings.Contains(answer, "Data Analytics con Power BI"),
	)
}
