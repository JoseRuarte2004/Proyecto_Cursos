# 🎓 Courses Platform -- Microservices Architecture

Sistema profesional de gestión de cursos online basado en arquitectura
de microservicios.

Diseñado para ser escalable, resiliente y orientado a producción.

------------------------------------------------------------------------

# 📌 Descripción General

La plataforma permite:

-   Registro y autenticación de usuarios
-   Diferenciación de roles (Alumno, Profesor, Administrador)
-   Creación y gestión completa de cursos (solo Admin)
-   Gestión estructural de clases (solo Admin)
-   Inscripción mediante pago integrado
-   Visualización de cursos adquiridos
-   Asignación de profesores a cursos
-   Panel administrativo avanzado
-   Arquitectura desacoplada y basada en eventos

------------------------------------------------------------------------

# 🏗 Arquitectura General

Frontend (React / Vue) ↓ Nginx (API Gateway / Load Balancer) ↓
Microservicios en Go ↓ PostgreSQL + Redis + RabbitMQ

------------------------------------------------------------------------

# 🧩 Microservicios

## 1️⃣ users-api

### Responsabilidades

-   Registro
-   Login
-   JWT
-   Gestión de roles (`student`, `teacher`, `admin`)
-   Acceso a datos sensibles (solo admin)
-   Promoción de usuario a profesor
-   Auditoría

### Base de datos

-   PostgreSQL
-   Redis (rate limiting)

------------------------------------------------------------------------

## 2️⃣ courses-api

### Responsabilidades

-   Crear, editar y eliminar cursos (solo admin)
-   Asignar profesores a cursos
-   Publicar/despublicar cursos
-   Gestionar cupos
-   Catálogo público

### Reglas de permisos

  Acción             Admin   Profesor           Alumno
  ------------------ ------- ------------------ --------
  Crear curso        ✅      ❌                 ❌
  Editar curso       ✅      ❌                 ❌
  Eliminar curso     ✅      ❌                 ❌
  Asignar profesor   ✅      ❌                 ❌
  Ver curso          ✅      ✅ (si asignado)   ✅

Un profesor puede estar asignado a múltiples cursos al mismo tiempo.

------------------------------------------------------------------------

## 3️⃣ course-content-api

Cada curso tiene una cantidad definida de clases creadas exclusivamente
por el Administrador.

### Modelo

Curso ├── Clase 1 ├── Clase 2 ├── Clase 3

### Cada clase incluye:

-   title
-   description
-   order
-   videoURL
-   recursos opcionales

### Reglas de permisos

  Acción           Admin   Profesor           Alumno
  ---------------- ------- ------------------ -------------------
  Crear clase      ✅      ❌                 ❌
  Editar clase     ✅      ❌                 ❌
  Eliminar clase   ✅      ❌                 ❌
  Ver clases       ✅      ✅ (si asignado)   ✅ (si inscripto)

El profesor no puede crear, modificar ni eliminar clases. Puede estar
asignado a múltiples cursos simultáneamente.

------------------------------------------------------------------------

## 4️⃣ enrollments-api

### Responsabilidades

-   Reserva de cupo
-   Confirmación tras pago
-   Listado de cursos del alumno
-   Listado de alumnos por curso
-   Vista global para administrador

### Permisos

  Acción                       Admin   Profesor           Alumno
  ---------------------------- ------- ------------------ --------
  Ver inscripciones globales   ✅      ❌                 ❌
  Ver inscriptos por curso     ✅      ✅ (si asignado)   ❌
  Ver mis cursos               ❌      ❌                 ✅

------------------------------------------------------------------------

## 5️⃣ payments-api

### Responsabilidades

-   Crear órdenes
-   Integración con pasarela (Stripe / MercadoPago)
-   Webhooks
-   Idempotencia
-   Publicación de eventos

### Eventos

-   payment.created
-   payment.paid
-   payment.failed
-   payment.refunded

------------------------------------------------------------------------

# 👥 Roles del Sistema

## 👨‍🎓 Alumno

-   Registrarse / Login
-   Comprar curso
-   Ver "Mis Cursos"
-   Acceder a clases si está inscripto

## 👨‍🏫 Profesor

-   Ver cursos asignados
-   Ver alumnos inscriptos
-   Ver contenido de clases
-   Puede estar asignado a múltiples cursos
-   No puede modificar cursos
-   No puede crear ni eliminar clases

## 👨‍💼 Administrador

-   CRUD completo de cursos
-   Crear/editar/eliminar clases
-   Asignar profesores
-   Ver todos los usuarios
-   Acceder a datos sensibles
-   Cambiar roles
-   Ver todas las inscripciones
-   Gestionar instancias del sistema

------------------------------------------------------------------------

# 🔐 Seguridad

-   JWT
-   Middleware RBAC
-   Contraseñas hasheadas (bcrypt)
-   Auditoría para datos sensibles
-   Validación estricta de permisos

------------------------------------------------------------------------

# 🚀 Escalabilidad y Resiliencia

-   Nginx como balanceador
-   RabbitMQ para eventos
-   Redis para alto rendimiento
-   PostgreSQL con índices
-   Health checks (`/health`, `/ready`)
-   Soporte para múltiples instancias

------------------------------------------------------------------------

# 🧪 Testing

-   Unit tests (services)
-   Mocks de repositorios
-   Integration tests
-   E2E con Docker Compose
-   Cobertura recomendada ≥ 70%

------------------------------------------------------------------------

# 🐳 Docker

-   Todos los servicios contenedorizados
-   Docker Compose
-   Variables de entorno
-   Repositorio en GitHub

------------------------------------------------------------------------

# 🎯 Objetivo Final

Construir una plataforma profesional, desacoplada, escalable y lista
para producción, aplicando principios modernos de arquitectura de
software.
