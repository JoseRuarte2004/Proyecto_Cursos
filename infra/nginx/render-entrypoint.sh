#!/bin/sh
set -eu

mkdir -p /etc/nginx/generated

LISTEN_PORT="${LISTEN_PORT:-${PORT:-10000}}"
FRONT_ORIGIN="${FRONT_ORIGIN:-https://app.example.com}"
USERS_API_UPSTREAM="${USERS_API_UPSTREAM:-http://users-api:8081}"
COURSES_API_UPSTREAM="${COURSES_API_UPSTREAM:-http://courses-api:8082}"
CONTENT_API_UPSTREAM="${CONTENT_API_UPSTREAM:-http://course-content-api:8083}"
ENROLLMENTS_API_UPSTREAM="${ENROLLMENTS_API_UPSTREAM:-http://enrollments-api:8084}"
PAYMENTS_API_UPSTREAM="${PAYMENTS_API_UPSTREAM:-http://payments-api:8085}"
CHAT_API_UPSTREAM="${CHAT_API_UPSTREAM:-http://chat-api:8090}"
SECURITY_TXT_CONTACT="${SECURITY_TXT_CONTACT:-mailto:security@example.com}"
SECURITY_TXT_EXPIRES="${SECURITY_TXT_EXPIRES:-2027-12-31T23:59:59Z}"
SECURITY_TXT_LANGUAGES="${SECURITY_TXT_LANGUAGES:-es, en}"
SECURITY_TXT_CANONICAL="${SECURITY_TXT_CANONICAL:-${PUBLIC_BASE_URL:-https://api.example.com}/.well-known/security.txt}"

CSP_VALUE="default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self'; connect-src 'self' wss://\$host https://\$host; img-src 'self' data: blob: https:; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com data:; media-src 'self' data: blob: https:; frame-src 'self' https://www.youtube.com https://player.vimeo.com; manifest-src 'self';"
CSP_HEADER_NAME="Content-Security-Policy"
HSTS_DIRECTIVE='add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;'

if [ "${APP_ENV:-production}" != "production" ]; then
  CSP_HEADER_NAME="Content-Security-Policy-Report-Only"
fi

if ! printf '%s' "${PUBLIC_BASE_URL:-}" | grep -Eq '^https://'; then
  HSTS_DIRECTIVE="# HSTS disabled because PUBLIC_BASE_URL is not HTTPS"
fi

export LISTEN_PORT FRONT_ORIGIN USERS_API_UPSTREAM COURSES_API_UPSTREAM CONTENT_API_UPSTREAM
export ENROLLMENTS_API_UPSTREAM PAYMENTS_API_UPSTREAM CHAT_API_UPSTREAM SECURITY_TXT_CONTACT
export SECURITY_TXT_EXPIRES SECURITY_TXT_LANGUAGES SECURITY_TXT_CANONICAL CSP_VALUE CSP_HEADER_NAME
export HSTS_DIRECTIVE

envsubst '${LISTEN_PORT} ${FRONT_ORIGIN} ${USERS_API_UPSTREAM} ${COURSES_API_UPSTREAM} ${CONTENT_API_UPSTREAM} ${ENROLLMENTS_API_UPSTREAM} ${PAYMENTS_API_UPSTREAM} ${CHAT_API_UPSTREAM} ${CSP_HEADER_NAME} ${CSP_VALUE} ${HSTS_DIRECTIVE}' \
  < /etc/nginx/templates/nginx.render.conf.template > /etc/nginx/nginx.conf

envsubst '${SECURITY_TXT_CONTACT} ${SECURITY_TXT_EXPIRES} ${SECURITY_TXT_LANGUAGES} ${SECURITY_TXT_CANONICAL}' \
  < /etc/nginx/templates/security.txt.template > /etc/nginx/generated/security.txt

exec nginx -g 'daemon off;'
