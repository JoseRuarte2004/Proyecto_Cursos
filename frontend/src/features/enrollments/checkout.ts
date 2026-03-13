export function buildCheckoutRedirectPath(input: {
  orderId: string;
  courseId: string;
  provider: string;
}) {
  const params = new URLSearchParams({
    orderId: input.orderId,
    courseId: input.courseId,
    provider: input.provider,
  });

  return `/checkout/redirect?${params.toString()}`;
}
