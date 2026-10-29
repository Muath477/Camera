# 03 — التحقق التقني من الخيارات الأساسية (أكتوبر 2026)

---

## 1. Frigate

### 1.1 الإصدار والترخيص
| البند | الحالة | المصدر |
|---|---|---|
| آخر إصدار مستقر | **0.17.2** (28 يونيو 2026): إصلاحات أمنية منها تجاوز صلاحيات الوصول للكاميرات عبر go2rtc لمستخدمي الأدوار المقيدة، وتسريب بيانات اعتماد RTSP للمشاهدين | [GitHub Releases](https://github.com/blakeblackshear/frigate/releases) |
| آخر إصدار تجريبي | **0.18.0-rc1** (30 أغسطس 2026): إعداد كامل من الواجهة، Profiles، دعم GenAI متعدد المزودين وllama.cpp، تضمينات بعيدة للبحث الدلالي، Motion Search | [GitHub Releases](https://github.com/blakeblackshear/frigate/releases) |
| تغييرات كاسرة في 0.18 | FFmpeg 8، لقطات WebP، صيغة جديدة للمناطق/الأقنعة — يلزم نسخ احتياطي للإعداد وقاعدة البيانات | [GitHub Releases](https://github.com/blakeblackshear/frigate/releases) |
| الترخيص | **MIT** — يسمح بالاستخدام التجاري والتعديل مع الإبقاء على إشعار الحقوق. اسم "Frigate" وشعاره **علامة تجارية** لشركة Frigate, Inc. وغير مشمولة بـ MIT | [GitHub](https://github.com/blakeblackshear/frigate) |
| النجوم | ~35.9 ألف | [GitHub](https://github.com/blakeblackshear/frigate) |

**أثر:** لا نستخدم اسم/شعار Frigate في واجهتنا أو تسويقنا. نماذج Frigate+ لها شروط منفصلة لم نتحقق منها **(غير مؤكد)**.

### 1.2 مواضيع MQTT التي سنستخدمها
المصدر لكل البنود: [Frigate MQTT docs](https://docs.frigate.video/integrations/mqtt)

| الموضوع | المحتوى | استخدامنا |
|---|---|---|
| `frigate/events` | JSON: `type` = `new` / `update` / `end` + `before` / `after`. الحقول: `id, camera, label, sub_label, start_time, end_time, score, box, area, current_zones, entered_zones, stationary, has_snapshot, has_clip, attributes, recognized_license_plate` | المصدر الرئيس لأحداث الكائنات → الوكيل يرفع حدثاً للسحابة عند `new`، ويحدّثه عند `end` |
| `frigate/reviews` | JSON بنفس النمط؛ الحقول: `id, camera, start_time, end_time, severity (alert/detection), thumb_path, data{detections, objects, sub_labels, zones, audio}` | الأنسب لإشعارات الجوال (عنصر مراجعة واحد يجمع عدة كائنات) — نُشعر عند `severity=alert` |
| `frigate/available` | `online` / `stopped` / `offline` (LWT) | مراقبة صحة الصندوق 24/7 (متطلب م3/4 من اللائحة) |
| `frigate/<camera>/<object>` | عدد الكائنات (رقم) | لوحات الإشغال/العدّ |
| `frigate/<camera>/<object>/snapshot` | JPEG ثنائي | لا نحتاجه غالباً؛ نجلب اللقطة عبر HTTP |
| `frigate/tracked_object_update` | `type`: `description` / `face` / `lpr` / `classification` | قراءة لوحات المركبات لمحطات الوقود والمواقف (مع مراعاة م10 من اللائحة) |

### 1.3 نقاط HTTP API التي سنستخدمها
| النقطة | الوظيفة | المصدر |
|---|---|---|
| `GET /api/events` (فلاتر: `before, after, cameras, labels, zones, limit, has_snapshot, has_clip, include_thumbnails, in_progress`) | مزامنة/استدراك الأحداث بعد انقطاع الاتصال | [Docs mirror](https://git.laurivan.com/Mirrors/blakeblackshear.frigate/src/commit/bd1d13d78c4ee7bcc5c45f431686a85548e47341/docs/docs/integrations/api.md)، [event.py](https://raw.githubusercontent.com/blakeblackshear/frigate/dev/frigate/api/event.py) |
| `GET /api/events/<id>` | تفاصيل حدث | نفسه |
| `GET /api/events/<id>/thumbnail.jpg` | الصورة المصغرة المرفوعة للسحابة | نفسه |
| `GET /api/events/<id>/snapshot.jpg` (`h, bbox, timestamp, crop, quality`) | لقطة كاملة للحدث | نفسه |
| `GET /api/events/<id>/clip.mp4` | المقطع القصير المرفوع للسحابة | نفسه |
| `POST` / `DELETE /api/events/<id>/retain` | **قفل/فك قفل الحدث من الحذف** ← ميزة "قفل الحوادث" (م6/1 لائحة) | نفسه |
| `GET /api/<camera>/latest.jpg` (`h, bbox, timestamp, zones, mask, motion, regions, quality`) | صورة حية للوحة والخريطة | [Docs mirror](https://git.laurivan.com/Mirrors/blakeblackshear.frigate/src/commit/bd1d13d78c4ee7bcc5c45f431686a85548e47341/docs/docs/integrations/api.md) |
| `GET /api/<camera>/recordings` (`after, before`) و`/recordings/summary` | خط زمني للتسجيلات وحساب مدة الحفظ الفعلية | نفسه |
| `GET /vod/<camera>/start/<ts>/end/<ts>/index.m3u8` | تشغيل التسجيل المحلي (HLS) عن بُعد عبر نفق | نفسه |
| `GET /api/events/search` (`query, search_type=thumbnail,description | similarity, event_id, cameras, labels, limit, after, before, ...`) | **البحث الدلالي** ("رجل بقميص أحمر") — يعيد 400 إذا البحث الدلالي معطل | [event.py](https://raw.githubusercontent.com/blakeblackshear/frigate/dev/frigate/api/event.py) |
| `GET /api/stats`, `/api/version`, `/api/config` | الصحة والإصدار | [Docs mirror](https://git.laurivan.com/Mirrors/blakeblackshear.frigate/src/commit/bd1d13d78c4ee7bcc5c45f431686a85548e47341/docs/docs/integrations/api.md) |
| `POST /api/login` (`user, password`) → JWT كوكي أو Bearer | المصادقة | [Auth docs](https://docs.frigate.video/configuration/authentication) |

### 1.4 المصادقة والمنافذ
- **8971**: واجهة وAPI مع مصادقة وأدوار (`admin` / `viewer` / مخصص لكاميرات محددة). **5000**: داخلي **بدون مصادقة** ويُعامل كل طلب كمدير ([Auth docs](https://docs.frigate.video/configuration/authentication)).
- **قرار:** الوكيل (Go) يتصل بـ 5000 عبر شبكة Docker الداخلية فقط؛ لا يُكشف أي منفذ للشبكة المحلية أو الإنترنت. كل الوصول البعيد عبر الوكيل والسحابة.

### 1.5 البحث الدلالي (Semantic Search)
- نماذج Jina CLIP v1 (إنجليزي، `small` يعمل على CPU) وv2 (89 لغة **تشمل العربية**، لكن يحتاج ذاكرة وGPU أكثر بكثير، وتحسين ~3% فقط) ([Docs](https://docs.frigate.video/configuration/semantic_search)).
- الحد الأدنى: **8GB RAM** ومعالج يدعم AVX/AVX2؛ الموصى به 16GB+ وGPU مخصص ([Docs](https://docs.frigate.video/configuration/semantic_search)).
- **أثر:** على N100 نشغّل v1 `small` فقط، والبحث بالعربية يُترجم في السحابة إلى الإنجليزية قبل الإرسال، أو نحسب التضمينات في السحابة (pgvector) من الصور المصغرة المرفوعة.

### 1.6 أداء العتاد للكشف
| العتاد | النموذج | زمن الاستدلال | المصدر |
|---|---|---|---|
| Intel N100 (OpenVINO) | MobileNetV2 | ~15ms | [Frigate hardware](https://docs.frigate.video/frigate/hardware) |
| Intel N100 (OpenVINO) | YOLOv9 s-320 | 30ms | نفسه |
| Intel N100 (OpenVINO) | YOLO-NAS 320 | ~25ms | نفسه |
| Hailo-8 | YOLOv6n | ~7ms | نفسه |
| Hailo-8L | YOLOv9-tiny 320 | 18ms | نفسه |

- N100 يشغّل **مثيلاً واحداً** للكاشف، ويكفي "عدة كاميرات 1080p بنشاط منخفض–متوسط" (Beelink EQ13 موصى به لوجود منفذَي شبكة) ([Frigate hardware](https://docs.frigate.video/frigate/hardware)).
- Coral **لم يعد موصى به** للتركيبات الجديدة ([Frigate hardware](https://docs.frigate.video/frigate/hardware)).
- **قرار مقترح:** N100 وحده حتى ~8 كاميرات بنشاط عادي؛ Hailo-8 كخيار للمواقع المزدحمة أو 9–16 كاميرا **(حدود الكاميرات تقدير — يجب قياسها ميدانياً)**.

---

## 2. go2rtc

| البند | الحالة | المصدر |
|---|---|---|
| آخر إصدار | **v1.9.14** (19 يناير 2026): إعادة كتابة منطق الاستماع لـ WebRTC، تحسينات ONVIF، محرر إعداد Monaco | [GitHub Releases](https://github.com/AlexxIT/go2rtc/releases) |
| إصدار أمني مهم | **v1.9.12** (نوفمبر 2025): `local_auth`، `allow_paths` لوحدات api/exec/echo، فحص المصادر غير الآمنة | [GitHub Releases](https://github.com/AlexxIT/go2rtc/releases) |
| الترخيص | MIT | [GitHub](https://github.com/AlexxIT/go2rtc) |
| المنافذ | API/واجهة 1984، RTSP 8554، WebRTC 8555 (TCP/UDP) | [GitHub](https://github.com/AlexxIT/go2rtc) |

### واجهة WebRTC وغيرها (OpenAPI 3.1، إصدار 1.9.13)
المصدر: [openapi.yaml](https://raw.githubusercontent.com/AlexxIT/go2rtc/master/website/api/openapi.yaml)
- `POST /api/webrtc?src={stream}` — **WHEP**: إرسال SDP offer (`application/sdp` خام أو JSON)، والرد SDP answer (200/201). **هذا ما سنستخدمه للبث الحي.**
- `POST /api/webrtc?dst={stream}` — **WHIP** للنشر (مثلاً الصوت ثنائي الاتجاه).
- `GET /api/ws` — WebSocket لتبادل الإشارات (`{type, value}`) مع `src`/`dst`.
- `GET/PUT/PATCH/DELETE /api/streams` — إدارة المصادر ديناميكياً (يستخدمها الوكيل عند إضافة كاميرا).
- `GET /api/frame.jpeg?src=` — لقطة؛ `GET /api/stream.mp4?src=` (MSE)؛ `GET /api/stream.m3u8?src=` (HLS).
- `GET/POST/PATCH /api/config` — **خطِر**: يعيد كتابة الإعداد.

**تحذيرات أمنية من المشروع نفسه:** الوصول إلى الـ API = خطر كامل لأن مصادر `exec`/`echo` تمنح تنفيذ أوامر؛ الطلبات من localhost تتجاوز المصادقة؛ المنافذ مفتوحة على الشبكة المحلية افتراضياً ([GitHub](https://github.com/AlexxIT/go2rtc)، [API README](https://github.com/AlexxIT/go2rtc/blob/master/internal/api/README.md)).

**قرار:** ربط go2rtc على `127.0.0.1`/شبكة Docker فقط، تعطيل `exec`/`echo` عبر `app.modules`، استخدام `api.allow_paths` بقائمة بيضاء، والبث الحي يمر عبر **TURN/SFU سحابي** بدلاً من فتح 8555 للإنترنت.

---

## 3. الثغرات المعروفة (CVE / GHSA)

### Frigate
| المعرف | الخطورة | الوصف | الإصدارات | المصدر |
|---|---|---|---|---|
| CVE-2023-45672 | RCE (الدرجة غير مؤكدة) | فك تسلسل YAML غير آمن ← تنفيذ تعليمات عن بُعد | أُصلحت في 0.13.0 Beta 3 | [GitHub Security Lab](https://github.blog/security/vulnerability-research/securing-our-home-labs-frigate-code-review/) |
| CVE-2023-45670 | CSRF (الدرجة غير مؤكدة) | CSRF في حفظ الإعداد | أُصلحت في 0.13.0 Beta 3 | نفسه |
| CVE-2023-45671 | — | (XSS حسب الترقيم، **غير مؤكد**) | أُصلحت في 0.13.0 Beta 3 | نفسه |
| CVE-2026-33124 (GHSA-24p8-r573-vwr2) | منخفضة | تغيير كلمة مرور غير آمن | أُصلحت في 0.17.0-beta1 | [Advisory](https://github.com/blakeblackshear/frigate/security/advisories/GHSA-24p8-r573-vwr2) |
| GHSA-74fj-g666-3rv9 | **عالية** | مستخدمو الأدوار المخصصة يستطيعون تخمين كلمة مرور أي حساب دون حد للمحاولات | **≤ 0.17.2؛ مُصلح في 0.18.0-rc1 فقط** | [Advisory](https://github.com/blakeblackshear/frigate/security/advisories/GHSA-74fj-g666-3rv9) |
| GHSA-cch2-w38h-8f44 | عالية (8.1) | وصول إداري بلا مصادقة بسبب `networking.listen.internal` | 0.18.0-beta1/2؛ مُصلح في beta3 | [Advisory](https://github.com/blakeblackshear/frigate/security/advisories/GHSA-cch2-w38h-8f44) |
| GHSA-h7qp-3pmh-jhmx، GHSA-vff9-46ww-fjjm، GHSA-2vw5-qcf8-4jm6 | متوسطة | تسريب بيانات بين الكاميرات خارج نطاق الدور (labels/وجوه/لوحات، motion search IDOR، ملخص GenAI) | 2026 | [Advisories](https://github.com/blakeblackshear/frigate/security/advisories) |
| GHSA-w3c4-957q-2jw4، GHSA-jqwr-vgw4-jmv7 | متوسطة | SSRF عبر Web Push | سبتمبر 2026 | نفسه |

ملاحظة من GitHub Security Lab: نقطة `/api/config/raw` غير المصادَق عليها (في الإصدارات القديمة) كانت تكشف كلمات مرور MQTT وRTSP ([GitHub Security Lab](https://github.blog/security/vulnerability-research/securing-our-home-labs-frigate-code-review/)).

### go2rtc
| المعرف | الخطورة | الوصف | المصدر |
|---|---|---|---|
| CVE-2024-29192 | عالية (8.8) | CSRF على `/api/config` ← إضافة مصدر `exec` ← تنفيذ أوامر | [cve.yack.one](https://cve.yack.one/vendors/alexxit) |
| CVE-2024-29193 (GO-2024-3053) | متوسطة (6.1) | XSS؛ مُصلح في 1.9.0 | [OSV](https://osv.dev/vulnerability/GO-2024-3053) |
| (CVE ثالث، 2024) | متوسطة (6.1) | XSS في صفحة الروابط؛ يطال ≤1.8.5 | [cve.yack.one](https://cve.yack.one/vendors/alexxit) |

**الخلاصة الأمنية:** Frigate وgo2rtc مصممان أساساً لبيئات منزلية موثوقة. نموذج التهديد لدينا (متعدد المستأجرين، صناديق لدى عملاء) يتطلب: عدم كشف أي منفذ لهما خارج الصندوق، تجميد الإصدارات مع تحديثات OTA موقعة، ومتابعة صفحة الـ Advisories أسبوعياً. **قرار مطلوب:** هل نعتمد 0.17.2 (مستقر لكن به ثغرة عالية غير مُصلحة إلا في 0.18) أم ننتظر 0.18.0 النهائي؟ التوصية: 0.18.x مع عزل الشبكة، لأن الثغرة تتطلب مستخدماً بدور مخصص على المنفذ 8971 الذي لن نكشفه أصلاً.

---

## 4. مناطق السحابة داخل المملكة

| المزوّد | المنطقة | الحالة | المصدر |
|---|---|---|---|
| Google Cloud | Dammam `me-central2` (منذ نوفمبر 2023) | متاحة؛ **العملاء السعوديون يشترون حصراً عبر CNTXT** (مشروع مشترك مع أرامكو)، والدعم الفني عبر CNTXT؛ خيارات Assured Workloads / KSA Data Boundary | [Google docs](https://docs.cloud.google.com/docs/dammam-region-access)، [Vision2030.ai](https://vision2030.ai/sectors/technology/saudi-arabia-cloud-regions/) |
| Microsoft Azure | Saudi Arabia East (المنطقة الشرقية، 3 مناطق توافر) | **مُعلن: نوفمبر 2026** | [Microsoft](https://news.microsoft.com/source/emea/2026/08/microsoft-announces-saudi-arabia-east-datacenter-region-will-be-available-in-november-2026/) |
| AWS | منطقة السعودية (استثمار 5.3 مليار دولار) | **"على المسار للإطلاق في ديسمبر 2026"** — غير متاحة بعد | [About Amazon](https://www.aboutamazon.com/news/aws/aws-cloud-region-saudi-arabia) |
| Oracle | جدة `me-jeddah-1` (2020)، الرياض `me-riyadh-1` (2024) | متاحة | [Vision2030.ai](https://vision2030.ai/sectors/technology/saudi-arabia-cloud-regions/) |
| Huawei Cloud | الرياض (2023، 3 مناطق توافر) | متاحة | نفسه |
| Alibaba Cloud (SCCC) | الرياض `me-central-1` (2022) | متاحة | نفسه |
| Tencent Cloud | الرياض (2025) | متاحة | نفسه |

**توصية:** لأن معماريتنا مبنية على مكونات مفتوحة (PostgreSQL/TimescaleDB/EMQX/Keycloak على Kubernetes)، نبدأ على **GCP Dammam عبر CNTXT** أو **Oracle Riyadh** الآن، ونُبقي النشر محايداً (Terraform + K8s) للانتقال إلى Azure/AWS عند إطلاقها. مزوّد محلي (STC Cloud وغيره) خيار للعملاء الحكوميين **(لم نتحقق من التفاصيل — غير مؤكد)**.

---

## 5. أسعار العتاد

| العنصر | السعر | المصدر |
|---|---|---|
| Beelink EQ14 (Intel N150، 16GB، 512GB، منفذا Gigabit) | ~189–200$ (≈ 710–750 ريال)، يناير 2025 | [Notebookcheck](https://www.notebookcheck.net/Beelink-EQ14-mini-PC-review-Ready-to-go-HTPC-for-under-200.939127.0.html) |
| Beelink EQ14/S13 Pro N150، 16GB، 1TB | 299 دولار أسترالي (عرض) | [OzBargain](https://ozbargain.com.au/node/888637) |
| Hailo-8 M.2 (26 TOPS، PCIe Gen3 x4، 2.5W نموذجي) | **179.99$** (≈ 675 ريال) | [Waveshare](https://www.waveshare.com/hailo-8.htm?sku=27841) |

- ملاحظات:
  - أسعار أجهزة N100/N150 لعام 2026 مرجّح أن تكون أعلى بسبب ارتفاع تكاليف الذاكرة والتخزين (وهو ما ذكرته Verkada سبباً لرفع أسعارها) ([Verkada](https://www.verkada.com/es/blog/2026-pricing-update/)) — **سعر 2026 داخل المملكة غير مؤكد ويحتاج عروض أسعار من موردين**.
  - EQ14 يدعم أقراص M.2 SATA فقط (حتى قرصين) ([Notebookcheck](https://www.notebookcheck.net/Beelink-EQ14-mini-PC-review-Ready-to-go-HTPC-for-under-200.939127.0.html))؛ القرص المخصص للتسجيل (HDD 3.5" للمراقبة) يتطلب هيكلاً يدعم SATA/حاوية خارجية — **يجب اختيار هيكل صناعي بمنفذ SATA وفتحة M.2 Key-M لـ Hailo**.
