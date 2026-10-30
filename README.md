# Camera

منصة لإدارة كاميرات المراقبة في المنشآت السعودية والامتثال لنظام استخدام كاميرات المراقبة الأمنية. جهاز حافة في كل موقع يستقبل أي كاميرا ويسجّل ويكشف محلياً، وسحابة داخل المملكة للهوية والأحداث والبحث والامتثال، وواجهة عربية.

الفيديو الكامل لا يغادر الموقع: تُرفع الأحداث والصور المصغرة والمقاطع القصيرة فقط، والبث الحي يذهب من الجهاز إلى المتصفح مباشرة.

- الخطة: [docs/plan.md](docs/plan.md)
- البحث (اللائحة، السوق، التحقق التقني، التسعير، مقارنة المنصات والتصاميم): [docs/research](docs/research)

## البنية

```
contract/          رسائل MQTT المشتركة بين الجهاز والسحابة
edge/agent/        وكيل الحافة: اكتشاف الكاميرات، إعداد Frigate، الأحداث، الصحة، الأوامر، الطابور الدائم
edge/profiles/     تعريفات علامات الكاميرات (مسارات RTSP، الواجهات، الشذوذات)
services/          وحدات السحابة: tenancy, devices, cameras, events, search, media, compliance, audit, auth, notify
services/db/       ترحيلات PostgreSQL (RLS، TimescaleDB، pgvector، PostGIS)
cmd/api/           خدمة السحابة (ومعها أوامر migrate و seed و vapid)
cmd/edge-agent/    وكيل الجهاز
cmd/device-sim/    جهاز افتراضي للتطوير
web/               الواجهة العربية (Next.js، MapLibre)
deploy/dev/        بيئة تطوير كاملة بـ Docker Compose
deploy/edge/       حزمة الحاويات على جهاز الحافة
deploy/prod/       النشر على خادم داخل المملكة (Caddy، شهادات الأجهزة، نسخ احتياطي)
```

## التشغيل المحلي

بالحاويات:

```bash
docker compose -f deploy/dev/docker-compose.yml up --build
docker compose -f deploy/dev/docker-compose.yml logs seed   # يطبع رمز دخول للتطوير
```

ثم افتح http://localhost:3000 والصق الرمز.

بدون حاويات (PostgreSQL 16 مع pgvector وPostGIS، ووسيط MQTT على 1883):

```bash
psql -U postgres -c "CREATE ROLE camera_api LOGIN PASSWORD 'camera_api'"
MIGRATE_DATABASE_URL=postgres://postgres:...@localhost:5432/camera go run ./cmd/api migrate
psql -U postgres -d camera -c "GRANT camera_app TO camera_api"

export DATABASE_URL=postgres://camera_api:camera_api@localhost:5432/camera DEV_JWT_SECRET=dev
go run ./cmd/api seed          # بيانات تجريبية ورمز دخول
go run ./cmd/api               # الخدمة على :8080
go run ./cmd/device-sim -tenant 11111111-1111-4111-8111-111111111111 -device 22222222-2222-4222-8222-000000000001

cd web && npm install && npm run dev
```

## الاختبارات

```bash
go test ./...
CAMERA_TEST_DATABASE_URL=postgres://camera_api:camera_api@localhost:5432/camera go test ./services/
cd web && npm run build
```

اختبارات القاعدة تتحقق من عزل المستأجرين ومن أن سجل التدقيق لا يُعدّل.

## إعدادات الخدمة

| المتغير | الغرض |
|---|---|
| `DATABASE_URL` | اتصال الخدمة (مستخدم غير مميّز عضو في `camera_app`) |
| `MIGRATE_DATABASE_URL` | اتصال المالك لتطبيق الترحيلات عند الإقلاع |
| `MQTT_URL`, `MQTT_USER`, `MQTT_PASSWORD` | وسيط الأجهزة |
| `APP_ENV` | `production` يفرض الأسرار ويمنع رموز التطوير |
| `APP_URL` | عنوان الواجهة الذي تفتحه الإشعارات |
| `JWT_SECRET` | توقيع رموز الجلسات بعد الدخول بالجوال (32 بايت على الأقل) |
| `DEV_JWT_SECRET` | بديل `JWT_SECRET` في التطوير فقط |
| `OIDC_JWKS_URL`, `OIDC_ISSUER`, `OIDC_AUDIENCE` | مزوّد هوية خارجي اختياري (Keycloak) |
| `SMS_PROVIDER`, `SMS_HTTP_URL`, `SMS_HTTP_TOKEN`, `SMS_FROM` | بوابة الرسائل النصية لرموز الدخول والتنبيهات (`http` أو `console`) |
| `WHATSAPP_TOKEN`, `WHATSAPP_PHONE_ID`, `WHATSAPP_TEMPLATE`, `WHATSAPP_LANG` | WhatsApp Business Cloud API (اختياري) |
| `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT` | إشعارات Web Push؛ أنشئها بـ `go run ./cmd/api vapid` |
| `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` | تخزين الوسائط |
| `S3_PUBLIC_URL` | العنوان العام للتخزين الذي تحمله الروابط الموقّعة |
| `SNAPSHOT_SIGNING_SEED` | مفتاح Ed25519 لتوقيع اللقطات (من KMS في الإنتاج) |
| `LINK_SECRET`, `ENROLL_SECRET` | روابط المدقق ورموز تفعيل الأجهزة |
| `EMBEDDER_URL` | خدمة التضمين النصي للبحث بالوصف |
| `TRUST_PROXY_DEVICE_IDENTITY` | قبول شهادة الجهاز من بوابة mTLS عبر الرأس `X-Client-Cert` |

## الدخول والإشعارات

- الدخول برقم الجوال: `POST /v1/auth/otp/request` يرسل رمزاً من ستة أرقام عبر رسالة نصية أو واتساب (ثلاث محاولات كل عشر دقائق)، و`POST /v1/auth/otp/verify` يعيد رمز جلسة لكل منشأة يملك الرقم عضوية فيها. رقم بلا حساب يسجّل منشأة جديدة عبر `POST /v1/auth/register`. في التطوير يعود الرمز في الاستجابة بدل الإرسال.
- التنبيهات على ثلاث درجات: الحرج يصل فوراً على القنوات المختارة (إشعار الجوال، رسالة نصية، واتساب)، والتحذير يحترم ساعات الهدوء، والعادي يبقى داخل التطبيق. الإعدادات في `GET/PUT /v1/me/notifications`، واشتراكات Web Push في `/v1/me/push-subscriptions`.
- الواجهة تطبيق ويب قابل للتثبيت (PWA) مع عامل خدمة يستقبل الإشعارات ويعرض صفحة بلا اتصال.
- الإقرار الجماعي بالأحداث مع ملاحظة في `POST /v1/events/ack`، وتقرير الامتثال بصيغة PDF عربية في `GET /v1/sites/{id}/compliance.pdf` وعبر رابط المدقق العام.

## النشر

`deploy/prod` يشغّل المنصة كاملة على خادم واحد داخل المملكة بـ Docker Compose: Caddy لشهادات HTTPS، وبوابة شهادات عميل لواجهة الأجهزة، وTimescaleDB، وMosquitto بشهادات من مرجع مصدّق خاص، وMinIO، ونسخ احتياطي ليلي. الخطوات في [deploy/prod/README.md](deploy/prod/README.md).

## الأمان

- كل جدول للمستأجر محمي بـ Row-Level Security، والخدمة تتصل بمستخدم ليس مالكاً ولا مميّزاً.
- بيانات اعتماد الكاميرات مشفّرة على الجهاز فقط.
- واجهة Frigate (المنفذ 5000) غير مكشوفة؛ الوكيل وحده يصل إليها.
- كل مشاهدة وتصدير ولقطة وتعديل يُسجَّل في سجل متسلسل التجزئة.
