export type Role = "admin" | "teacher" | "student";

export type MediaAttachment = {
  id: string;
  kind: "image" | "video" | "file";
  fileName: string;
  contentType: string;
  sizeBytes: number;
  url: string;
  createdAt: string;
};

export type ApiErrorEnvelope = {
  error?: {
    code?: string;
    message?: string;
    details?: unknown;
  };
  requestId?: string;
};

export type AuthUser = {
  id: string;
  name: string;
  email: string;
  role: Role;
};

export type LoginResponse = {
  token: string;
  user: AuthUser;
};

export type Course = {
  id: string;
  title: string;
  description: string;
  category: string;
  imageUrl?: string | null;
  price: number;
  currency: string;
  capacity: number;
  status: "draft" | "published";
  createdBy: string;
  createdAt: string;
  updatedAt: string;
};

export type CourseRecommendation = {
  answer: string;
};

export type CourseRecommendationMessage = {
  role: "user" | "assistant";
  content: string;
};

export type Lesson = {
  id: string;
  courseId: string;
  title: string;
  description: string;
  orderIndex: number;
  videoUrl: string;
  attachments: MediaAttachment[];
  createdAt: string;
  updatedAt: string;
};

export type EnrollmentStatus = "pending" | "active" | "cancelled" | "refunded";

export type Enrollment = {
  id: string;
  userId: string;
  courseId: string;
  status: EnrollmentStatus;
  createdAt: string;
};

export type TeacherCourseEnrollment = {
  studentName: string;
  status: EnrollmentStatus;
  createdAt: string;
};

export type TeacherCourseEnrollmentsResponse = {
  courseId: string;
  courseTitle: string;
  enrollments: TeacherCourseEnrollment[];
};

export type AdminEnrollment = {
  studentName: string;
  courseTitle: string;
  status: EnrollmentStatus;
  createdAt: string;
};

export type MyEnrollment = {
  courseId: string;
  status: EnrollmentStatus;
  createdAt: string;
  course: {
    id: string;
    title: string;
    category: string;
    imageUrl?: string | null;
    price: number;
    currency: string;
    status: string;
  };
};

export type Availability = {
  courseId: string;
  capacity: number;
  activeCount: number;
  available: number;
};

export type AdminUser = {
  id: string;
  name: string;
  email: string;
  role: Role;
  createdAt: string;
  updatedAt: string;
};

export type AdminUserDetail = AdminUser & {
  phone: string;
  dni: string;
  address: string;
};

export type OrderResult = {
  orderId: string;
  checkoutUrl: string;
  provider: "mercadopago" | "stripe";
  idempotencyKey: string;
  status: "created" | "pending" | "paid" | "failed" | "refunded";
  providerStatus?: string;
  providerPaymentId?: string;
  providerPreferenceId?: string;
  externalReference?: string;
  paidAt?: string;
  failedAt?: string;
  lastWebhookAt?: string;
};

export type TeacherListResponse = {
  teacherIds: string[];
};

export type ChatMessage = {
  id: string;
  roomId?: string;
  courseId: string;
  senderId: string;
  senderName?: string;
  senderRole: Role;
  content: string;
  attachments: MediaAttachment[];
  createdAt: string;
};

export type ChatPrivateContact = {
  userId: string;
  name: string;
  role: Role;
};

export type ChatHello = {
  type: "hello";
  courseId: string;
  userId: string;
  role: Role;
};

export type ChatRealtimeMessage = {
  type: "message";
} & ChatMessage;

export type ChatRealtimeError = {
  type: "error";
  code: string;
  message: string;
  requestId?: string;
};
