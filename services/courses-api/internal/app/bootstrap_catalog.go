package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/services/courses-api/internal/domain"
)

const bootstrapCatalogOwnerID = "00000000-0000-0000-0000-000000000001"

type bootstrapCatalogRepository interface {
	CountCourses(ctx context.Context) (int, error)
	Create(ctx context.Context, course domain.Course) (*domain.Course, error)
}

type bootstrapCatalogSeed struct {
	Title       string
	Description string
	Category    string
	Price       float64
	Currency    string
	Capacity    int
}

var defaultBootstrapCatalog = []bootstrapCatalogSeed{
	{
		Title:       "Desarrollo Web Desde Cero",
		Description: "Aprende a crear paginas web modernas desde cero con HTML, CSS, estructura semantica, responsive design y buenas practicas para publicar tus primeros proyectos con una base solida.",
		Category:    "frontend",
		Price:       15000,
		Currency:    "ARS",
		Capacity:    50,
	},
	{
		Title:       "HTML y CSS Practico",
		Description: "Maqueta interfaces responsivas, mejora estilos, organiza componentes visuales y construye paginas claras y prolijas con un enfoque completamente practico y orientado a portfolio.",
		Category:    "frontend",
		Price:       12000,
		Currency:    "ARS",
		Capacity:    50,
	},
	{
		Title:       "JavaScript Inicial",
		Description: "Domina variables, funciones, eventos, arrays, objetos y manipulacion del DOM para agregar interactividad real a sitios web y sentar una base solida para frontend.",
		Category:    "frontend",
		Price:       18000,
		Currency:    "ARS",
		Capacity:    50,
	},
	{
		Title:       "Backend con Go",
		Description: "Construye APIs, rutas, middlewares y servicios backend con Go para entender como conectar frontend con logica de negocio real, validaciones y acceso a datos.",
		Category:    "backend",
		Price:       22000,
		Currency:    "ARS",
		Capacity:    40,
	},
	{
		Title:       "Finanzas Personales y Presupuesto",
		Description: "Aprende a ordenar ingresos, gastos, ahorro y fondo de emergencia con un metodo practico que te ayude a tomar decisiones financieras mas claras en tu vida diaria.",
		Category:    "finanzas",
		Price:       16000,
		Currency:    "ARS",
		Capacity:    60,
	},
	{
		Title:       "Analisis Financiero para Emprendedores",
		Description: "Interpreta flujo de caja, costos, rentabilidad, margen, punto de equilibrio e indicadores basicos para evaluar proyectos, negocios y decisiones comerciales con criterio financiero.",
		Category:    "finanzas",
		Price:       24000,
		Currency:    "ARS",
		Capacity:    45,
	},
	{
		Title:       "Introduccion al Derecho Laboral",
		Description: "Entiende derechos y obligaciones en relaciones laborales, tipos de contrato, jornada, licencias, remuneracion y desvinculacion para moverte con mayor seguridad juridica.",
		Category:    "derecho",
		Price:       21000,
		Currency:    "ARS",
		Capacity:    45,
	},
	{
		Title:       "Contratos Comerciales para Pymes",
		Description: "Revisa clausulas clave, riesgos frecuentes y buenas practicas para leer, redactar y negociar contratos comerciales con un enfoque claro para emprendimientos y pymes.",
		Category:    "derecho",
		Price:       26000,
		Currency:    "ARS",
		Capacity:    35,
	},
	{
		Title:       "Excel Aplicado a Gestion",
		Description: "Domina formulas, tablas dinamicas, validaciones, limpieza de datos y reportes operativos para mejorar el analisis y control de informacion en contextos administrativos.",
		Category:    "herramientas",
		Price:       14000,
		Currency:    "ARS",
		Capacity:    70,
	},
	{
		Title:       "Marketing Digital y Redes Sociales",
		Description: "Diseña campañas, contenidos y estrategias basicas para captar clientes, ordenar publicaciones y mejorar tu presencia digital en redes y canales de venta online.",
		Category:    "marketing",
		Price:       19000,
		Currency:    "ARS",
		Capacity:    60,
	},
	{
		Title:       "Gestion de Proyectos con Scrum",
		Description: "Organiza proyectos con backlog, roles, sprints, ceremonias y seguimiento para coordinar equipos de forma agil y dar visibilidad al avance de cada entrega.",
		Category:    "gestion",
		Price:       23000,
		Currency:    "ARS",
		Capacity:    40,
	},
	{
		Title:       "Data Analytics con Power BI",
		Description: "Aprende a modelar datos, crear dashboards y comunicar indicadores con Power BI para transformar informacion dispersa en decisiones claras y accionables.",
		Category:    "data",
		Price:       25000,
		Currency:    "ARS",
		Capacity:    40,
	},
	{
		Title:       "Python para Analisis de Datos",
		Description: "Trabaja con Python, estructuras de datos y librerias introductorias para limpiar informacion, automatizar tareas repetitivas y empezar a analizar datasets reales.",
		Category:    "data",
		Price:       23500,
		Currency:    "ARS",
		Capacity:    45,
	},
}

func EnsureBootstrapCatalog(ctx context.Context, repo bootstrapCatalogRepository, log *logger.Logger) error {
	if repo == nil {
		return nil
	}

	count, err := repo.CountCourses(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	now := time.Now().UTC()
	for _, seed := range defaultBootstrapCatalog {
		_, err := repo.Create(ctx, domain.Course{
			ID:          uuid.NewString(),
			Title:       seed.Title,
			Description: seed.Description,
			Category:    seed.Category,
			Price:       seed.Price,
			Currency:    seed.Currency,
			Capacity:    seed.Capacity,
			Status:      domain.StatusPublished,
			CreatedBy:   bootstrapCatalogOwnerID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		if err != nil {
			return err
		}
	}

	if log != nil {
		log.Info(context.Background(), "bootstrap catalog created", map[string]any{
			"courses": len(defaultBootstrapCatalog),
		})
	}

	return nil
}
