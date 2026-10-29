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
services/          وحدات السحابة: tenancy, devices, cameras, events, search, media, compliance, audit
services/db/       ترحيلات PostgreSQL (RLS، TimescaleDB، pgvector، PostGIS)
cmd/api/           خدمة السحابة (ومعها أوامر migrate و seed)
cmd/edge-agent/    وكيل الجهاز
cmd/device-sim/    جهاز افتراضي للتطوير
web/               الواجهة العربية (Next.js، MapLibre)
deploy/dev/        بيئة تطوير كاملة بـ Docker Compose
deploy/edge/       حزمة الحاويات على جهاز الحافة
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
| `OIDC_JWKS_URL`, `OIDC_ISSUER`, `OIDC_AUDIENCE` | التحقق من رموز Keycloak |
| `DEV_JWT_SECRET` | رموز التطوير فقط |
| `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` | تخزين الوسائط |
| `SNAPSHOT_SIGNING_SEED` | مفتاح Ed25519 لتوقيع اللقطات (من KMS في الإنتاج) |
| `LINK_SECRET`, `ENROLL_SECRET` | روابط المدقق ورموز تفعيل الأجهزة |
| `EMBEDDER_URL` | خدمة التضمين النصي للبحث بالوصف |
| `TRUST_PROXY_DEVICE_IDENTITY` | قبول هوية الجهاز من بوابة mTLS |

## الأمان

- كل جدول للمستأجر محمي بـ Row-Level Security، والخدمة تتصل بمستخدم ليس مالكاً ولا مميّزاً.
- بيانات اعتماد الكاميرات مشفّرة على الجهاز فقط.
- واجهة Frigate (المنفذ 5000) غير مكشوفة؛ الوكيل وحده يصل إليها.
- كل مشاهدة وتصدير ولقطة وتعديل يُسجَّل في سجل متسلسل التجزئة.
