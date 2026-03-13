import { apiRequest } from "@/api/client";
import type {
  AdminEnrollment,
  AdminUser,
  AdminUserDetail,
  AuthUser,
  Availability,
  ChatMessage,
  ChatPrivateContact,
  Course,
  CourseRecommendation,
  CourseRecommendationMessage,
  Enrollment,
  Lesson,
  LoginResponse,
  MyEnrollment,
  OrderResult,
  TeacherCourseEnrollmentsResponse,
  TeacherListResponse,
} from "@/api/types";

function normalizeLesson(lesson: Lesson): Lesson {
  return {
    ...lesson,
    attachments: lesson.attachments ?? [],
  };
}

function normalizeLessons(lessons: Lesson[]): Lesson[] {
  return lessons.map(normalizeLesson);
}

function normalizeChatMessage(message: ChatMessage): ChatMessage {
  return {
    ...message,
    attachments: message.attachments ?? [],
  };
}

function normalizeChatMessages(messages: ChatMessage[]): ChatMessage[] {
  return messages.map(normalizeChatMessage);
}

function buildLessonBody(
  input: {
    title?: string;
    description?: string;
    orderIndex?: number;
    videoUrl?: string;
    attachments?: File[];
  },
) {
  if (input.attachments?.length) {
    const formData = new FormData();
    if (typeof input.title === "string") {
      formData.append("title", input.title);
    }
    if (typeof input.description === "string") {
      formData.append("description", input.description);
    }
    if (typeof input.orderIndex === "number") {
      formData.append("orderIndex", String(input.orderIndex));
    }
    if (typeof input.videoUrl === "string") {
      formData.append("videoUrl", input.videoUrl);
    }
    input.attachments.forEach((file) => formData.append("attachments", file));
    return formData;
  }

  return JSON.stringify(input);
}

function buildMessageBody(input: { content: string; attachments?: File[] }) {
  if (input.attachments?.length) {
    const formData = new FormData();
    formData.append("content", input.content);
    input.attachments.forEach((file) => formData.append("attachments", file));
    return formData;
  }

  return JSON.stringify({ content: input.content });
}

export const usersApi = {
  register: (input: {
    name: string;
    email: string;
    password: string;
    phone?: string;
    dni?: string;
    address?: string;
  }) =>
    apiRequest<AuthUser>("/users/auth/register", {
      method: "POST",
      auth: false,
      timeoutMs: 15000,
      body: JSON.stringify(input),
    }),
  registerWithCode: (input: {
    name: string;
    email: string;
    password: string;
    phone?: string;
    dni?: string;
    address?: string;
  }) =>
    apiRequest<AuthUser>("/users/register", {
      method: "POST",
      auth: false,
      timeoutMs: 15000,
      body: JSON.stringify(input),
    }),
  verifyCode: (input: { email: string; code: string }) =>
    apiRequest<{ status: string }>("/users/verify", {
      method: "POST",
      auth: false,
      body: JSON.stringify(input),
    }),
  requestPasswordResetCode: (input: { email: string }) =>
    apiRequest<{ status: string }>("/users/auth/password/forgot/code", {
      method: "POST",
      auth: false,
      timeoutMs: 15000,
      body: JSON.stringify(input),
    }),
  resetPasswordWithCode: (input: {
    email: string;
    code: string;
    newPassword: string;
  }) =>
    apiRequest<{ status: string }>("/users/auth/password/reset/code", {
      method: "POST",
      auth: false,
      body: JSON.stringify(input),
    }),
  login: (input: { email: string; password: string }) =>
    apiRequest<LoginResponse>("/users/auth/login", {
      method: "POST",
      auth: false,
      body: JSON.stringify(input),
    }),
  me: () => apiRequest<AuthUser>("/users/me"),
  listAdminUsers: () => apiRequest<AdminUser[]>("/users/admin/users"),
  getAdminUser: (id: string) =>
    apiRequest<AdminUserDetail>(`/users/admin/users/${id}`),
  changeRole: (id: string, role: AuthUser["role"]) =>
    apiRequest<AuthUser>(`/users/admin/users/${id}/role`, {
      method: "PATCH",
      body: JSON.stringify({ role }),
    }),
};

export const coursesApi = {
  listPublished: (limit = 9, offset = 0) =>
    apiRequest<Course[]>(`/courses/courses?limit=${limit}&offset=${offset}`, {
      auth: false,
    }),
  recommend: (input: {
    name?: string;
    question: string;
    history?: CourseRecommendationMessage[];
  }) =>
    apiRequest<CourseRecommendation>("/courses/recommend", {
      method: "POST",
      auth: false,
      timeoutMs: 30000,
      body: JSON.stringify(input),
    }),
  getPublished: (id: string) =>
    apiRequest<Course>(`/courses/courses/${id}`, {
      auth: false,
    }),
  create: (input: {
    title: string;
    description: string;
    category: string;
    imageUrl?: string | null;
    price: number;
    currency: string;
    capacity: number;
    status: Course["status"];
  }) =>
    apiRequest<Course>("/courses/courses", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  update: (
    id: string,
    input: Partial<{
      title: string;
      description: string;
      category: string;
      imageUrl: string | null;
      price: number;
      currency: string;
      capacity: number;
      status: Course["status"];
    }>,
  ) =>
    apiRequest<Course>(`/courses/courses/${id}`, {
      method: "PATCH",
      body: JSON.stringify(input),
    }),
  delete: (id: string) =>
    apiRequest<void>(`/courses/courses/${id}`, {
      method: "DELETE",
    }),
  assignTeacher: (courseId: string, teacherId: string) =>
    apiRequest<{ status: string }>(`/courses/courses/${courseId}/teachers`, {
      method: "POST",
      body: JSON.stringify({ teacherId }),
    }),
  listTeachers: (courseId: string) =>
    apiRequest<TeacherListResponse>(`/courses/courses/${courseId}/teachers`),
  removeTeacher: (courseId: string, teacherId: string) =>
    apiRequest<void>(`/courses/courses/${courseId}/teachers/${teacherId}`, {
      method: "DELETE",
    }),
  teacherCourses: () => apiRequest<Course[]>("/courses/teacher/me/courses"),
};

export const contentApi = {
  listLessons: (courseId: string) =>
    apiRequest<Lesson[]>(`/content/courses/${courseId}/lessons`).then(normalizeLessons),
  createLesson: (
    courseId: string,
    input: {
      title: string;
      description: string;
      orderIndex: number;
      videoUrl: string;
      attachments?: File[];
    },
  ) =>
    apiRequest<Lesson>(`/content/courses/${courseId}/lessons`, {
      method: "POST",
      body: buildLessonBody(input),
    }).then(normalizeLesson),
  updateLesson: (
    courseId: string,
    lessonId: string,
    input: Partial<{
      title: string;
      description: string;
      orderIndex: number;
      videoUrl: string;
      attachments: File[];
    }>,
  ) =>
    apiRequest<Lesson>(`/content/courses/${courseId}/lessons/${lessonId}`, {
      method: "PATCH",
      body: buildLessonBody(input),
    }).then(normalizeLesson),
  deleteLesson: (courseId: string, lessonId: string) =>
    apiRequest<void>(`/content/courses/${courseId}/lessons/${lessonId}`, {
      method: "DELETE",
    }),
};

export const enrollmentsApi = {
  availability: (courseId: string) =>
    apiRequest<Availability>(`/enrollments/courses/${courseId}/availability`, {
      auth: false,
    }),
  reserve: (courseId: string) =>
    apiRequest<Enrollment>("/enrollments/enrollments/reserve", {
      method: "POST",
      body: JSON.stringify({ courseId }),
    }),
  myEnrollments: () => apiRequest<MyEnrollment[]>("/enrollments/me/enrollments"),
  adminEnrollments: (limit = 50, offset = 0) =>
    apiRequest<AdminEnrollment[]>(
      `/enrollments/admin/enrollments?limit=${limit}&offset=${offset}`,
    ),
  teacherCourseEnrollments: (courseId: string) =>
    apiRequest<TeacherCourseEnrollmentsResponse>(
      `/enrollments/teacher/courses/${courseId}/enrollments`,
    ),
};

export const paymentsApi = {
  createOrder: (input: { courseId: string; provider: "mercadopago" | "stripe" }) =>
    apiRequest<OrderResult>("/payments/orders", {
      method: "POST",
      headers: {
        "Idempotency-Key": crypto.randomUUID(),
      },
      body: JSON.stringify(input),
    }),
  getOrder: (
    orderId: string,
    options?: {
      paymentId?: string;
    },
  ) => {
    const paymentId = options?.paymentId?.trim();
    const query = paymentId ? `?paymentId=${encodeURIComponent(paymentId)}` : "";
    return apiRequest<OrderResult>(`/payments/orders/${orderId}${query}`);
  },
};

export const chatApi = {
  listMessages: (courseId: string, limit = 50, before?: string) =>
    apiRequest<ChatMessage[]>(
      `/chat/courses/${courseId}/messages?limit=${limit}${before ? `&before=${encodeURIComponent(before)}` : ""}`,
    ).then(normalizeChatMessages),
  sendMessage: (courseId: string, input: { content: string; attachments?: File[] }) =>
    apiRequest<ChatMessage>(`/chat/courses/${courseId}/messages`, {
      method: "POST",
      body: buildMessageBody(input),
    }).then(normalizeChatMessage),
  listPrivateContacts: (courseId: string) =>
    apiRequest<ChatPrivateContact[]>(`/chat/courses/${courseId}/private/contacts`),
  listPrivateMessages: (
    courseId: string,
    otherUserId: string,
    limit = 50,
    before?: string,
  ) =>
    apiRequest<ChatMessage[]>(
      `/chat/courses/${courseId}/private/${otherUserId}/messages?limit=${limit}${before ? `&before=${encodeURIComponent(before)}` : ""}`,
    ).then(normalizeChatMessages),
  sendPrivateMessage: (
    courseId: string,
    otherUserId: string,
    input: { content: string; attachments?: File[] },
  ) =>
    apiRequest<ChatMessage>(`/chat/courses/${courseId}/private/${otherUserId}/messages`, {
      method: "POST",
      body: buildMessageBody(input),
    }).then(normalizeChatMessage),
};
