# النشر على خادم واحد داخل المملكة

حزمة Docker Compose تشغّل المنصة كاملة على خادم افتراضي واحد (4 أنوية، 8 جيجابايت ذاكرة، قرص 200 جيجابايت يكفي للبداية) عند مزوّد سحابة داخل المملكة. تناسب أول مئات المواقع؛ بعدها تُفصل القاعدة والتخزين إلى خدمات مُدارة بنفس المتغيرات.

المكوّنات:

| الخدمة | الدور | المنفذ العام |
|---|---|---|
| caddy | HTTPS للتطبيق والواجهة والوسائط، وبوابة شهادات العميل لواجهة الأجهزة | 80, 443 |
| postgres | TimescaleDB على PostgreSQL 16 (بيانات المستأجرين، RLS) | لا |
| mosquitto | وسيط MQTT للأجهزة بشهادات عميل، وللخدمة بكلمة مرور | 8883 |
| minio | الصور المصغرة والمقاطع مع حذف تلقائي بعد مدة الاحتفاظ | عبر caddy فقط |
| api | خدمة السحابة (ترحيل القاعدة عند الإقلاع) | عبر caddy فقط |
| web | الواجهة العربية | عبر caddy فقط |
| backup | نسخة احتياطية ليلية للقاعدة | لا |

## 1. المتطلبات

- Ubuntu 24.04 مع Docker Engine وملحق Compose (`docker compose version` يعمل).
- خمسة أسماء نطاق تشير إلى عنوان الخادم (سجلات A): `app.` و`api.` و`media.` لشهادات Let's Encrypt التلقائية، و`mqtt.` و`device.` بشهادة من المرجع المصدّق الخاص الذي تثق به الأجهزة. يجب أن يكون المنفذان 80 و443 مفتوحين للعالم لإصدار الشهادات.
- جدار الحماية يفتح 80 و443 و8883 و22 فقط.
- بوابة رسائل نصية سعودية مرخّصة (مثل Unifonic أو Msegat) أو حساب WhatsApp Business، لإرسال رموز الدخول والتنبيهات.

## 2. الإعداد

```bash
git clone https://github.com/Muath477/Camera.git && cd Camera/deploy/prod
cp .env.example .env
```

افتح `.env` وعبّئ كل متغير:

- النطاقات والبريد الإلكتروني لشهادات Let's Encrypt.
- الأسرار: `openssl rand -base64 32` لكل من `POSTGRES_PASSWORD` و`CAMERA_API_PASSWORD` و`MQTT_PASSWORD` و`S3_SECRET_KEY` و`JWT_SECRET` و`LINK_SECRET` و`ENROLL_SECRET` و`SNAPSHOT_SIGNING_SEED`.
- مفاتيح الإشعارات: `go run ./cmd/api vapid` من جذر المستودع يطبع `VAPID_PUBLIC_KEY` و`VAPID_PRIVATE_KEY`. احتفظ بهما؛ تغييرهما يُسقط اشتراكات الإشعارات على كل الأجهزة.
- بوابة الرسائل: `SMS_PROVIDER=http` مع `SMS_HTTP_URL` و`SMS_HTTP_TOKEN`. الخدمة ترسل `POST` بجسم `{"to","from","text"}` ورأس `Authorization: Bearer`. لبوابات بصيغة مختلفة ضع وسيطاً صغيراً أمامها أو أضف مرسلاً في `services/notify/senders.go`.
- واتساب (اختياري): رمز وصول دائم من Meta Business، ومعرّف رقم الهاتف، واسم قالب معتمد بمتغير واحد في النص.

ثم هيئة المرجع المصدّق للأجهزة وكلمة مرور الخدمة على الوسيط:

```bash
./pki/init-ca.sh mqtt.example.sa device.example.sa
docker run --rm -v ./mosquitto:/m eclipse-mosquitto:2.0 \
  mosquitto_passwd -c -b /m/passwd api "$(grep ^MQTT_PASSWORD= .env | cut -d= -f2)"
```

`pki/out/ca.key` يوقّع شهادات الأجهزة؛ انسخه إلى مدير كلمات المرور واحتفظ به خارج الخادم بعد الإعداد. لا شيء من `pki/out` أو `.env` أو `mosquitto/passwd` يُرفع إلى المستودع.

## 3. التشغيل

```bash
docker compose up -d --build
docker compose logs -f api        # "migrations applied" ثم "api listening"
curl -s https://api.example.sa/healthz   # ok
```

صورة الخدمة تُسحب من `ghcr.io/muath477/camera-api` (تُبنى من `main` بالتكامل المستمر)، والواجهة تُبنى على الخادم لأن عنوان الواجهة البرمجية يُثبَّت فيها وقت البناء.

افتح `https://app.example.sa`، أدخل رقم جوالك، واستلم الرمز. أول رقم بلا حساب يُسجّل منشأة جديدة ويصبح مالكها. إن لم تصل الرسائل راجع `docker compose logs api` حيث تظهر أخطاء البوابة.

## 4. إضافة جهاز حافة

1. من الواجهة: الأجهزة، إضافة جهاز. أدخل الرقم التسلسلي ورمز التفعيل المطبوعين على الجهاز؛ تظهر معرّفات الجهاز والمنشأة.
2. على الخادم أصدر شهادة الجهاز:

   ```bash
   ./pki/issue-device.sh <device-id> <tenant-id>
   ```

   تُكتب `pki/out/devices/<device-id>.crt` و`.key`. انسخهما مع `pki/out/ca.crt` إلى `/etc/camera/` على الجهاز.
3. على الجهاز عدّل `/etc/camera/agent.yaml` (نموذج في `deploy/edge/agent.example.yaml`):

   ```yaml
   device_id: <device-id>
   tenant_id: <tenant-id>
   cloud:
     broker_url: ssl://mqtt.example.sa:8883
     api_url: https://device.example.sa
     ca_file: /etc/camera/ca.crt
     cert_file: /etc/camera/<device-id>.crt
     key_file: /etc/camera/<device-id>.key
   ```

4. شغّل حزمة الحافة (`deploy/edge`). الوسيط يأخذ اسم المستخدم من الشهادة، وقائمة التحكم تحصر كل جهاز في مواضيعه `t/<tenant>/d/<device>/#`. رفع الصور والمقاطع يمر عبر `device.example.sa` بنفس الشهادة ثم مباشرة إلى التخزين عبر `media.example.sa`.

## 5. التشغيل اليومي

- **التحديث**: `docker compose pull api && docker compose up -d --build`. الترحيلات تُطبَّق عند إقلاع الخدمة.
- **النسخ الاحتياطي**: تفريغ ليلي في وحدة التخزين `backups` مع الاحتفاظ `BACKUP_KEEP_DAYS` يوماً. اضبط لقطات القرص عند المزوّد إلى منطقة أخرى داخل المملكة، فالنسخة على نفس القرص لا تكفي.
- **الاستعادة**: `docker compose exec -T postgres pg_restore -U postgres -d camera --clean < camera-YYYYMMDD-HHMM.dump`.
- **السجلات**: `docker compose logs --since 1h api` بصيغة JSON؛ وجّهها إلى مجمّع سجلات عند الحاجة.
- **المراقبة**: `/healthz` يتحقق من القاعدة. أضف فحصاً خارجياً عليه وعلى المنفذ 8883.
- **تدوير الأسرار**: تغيير `JWT_SECRET` يُخرج كل المستخدمين من التطبيق مرة واحدة ولا يؤثر على الأجهزة. تغيير `SNAPSHOT_SIGNING_SEED` يُبطل التحقق من اللقطات القديمة، فاحتفظ بالمفتاح السابق في الأرشيف.

## 6. ما يبقى خارج هذه الحزمة

- خدمة التضمين النصي للبحث بالوصف (`EMBEDDER_URL`) تحتاج معالج رسومي أو خدمة خارجية داخل المملكة.
- مزوّد هوية خارجي (Keycloak) اختياري عبر `OIDC_*`؛ الدخول برقم الجوال يكفي للمنشآت الصغيرة والمتوسطة.
- عند تجاوز خادم واحد: انقل القاعدة إلى خدمة مُدارة بنفس `DATABASE_URL`، والوسائط إلى تخزين كائنات مُدار بنفس متغيرات `S3_*`، وشغّل نسختين من `api` خلف Caddy.
