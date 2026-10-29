# 00 — الملخص التنفيذي: بحث منصة Camera

> أكتوبر 2026. التفاصيل والمصادر في الملفات 01–04. كل ما لم يُتحقق منه من نص رسمي موسوم **(غير مؤكد)**.

## الخلاصة في 5 أسطر
1. **السوق إلزامي:** نظام استخدام كاميرات المراقبة الأمنية (م/34، 2022) ولائحته (يونيو 2023) يُلزمان صراحةً معظم شرائحنا: منافذ الأطعمة، الفنادق، المجمعات التجارية، المستودعات التجارية، محطات الوقود، المباني السكنية ([يمتاز](https://ymtaz.sa/law-guide/71)). الشهادة شرط لإصدار/تجديد الرخصة (م4 لائحة) ([أم القرى](https://www.uqn.gov.sa/details?p=23050)).
2. **مدة الحفظ تحددها "وثيقة الشروط"** وليس النظام. الرقم المتداول **90 يوماً (غير مؤكد)**، مع ذكر 31 و120 في مصادر أخرى؛ والحادثة المبلغ عنها تُحفظ حتى انتهاء التحقيق أو المدة، أيهما أطول (م6 لائحة).
3. **السوق** ~433 مليون دولار (2025) بنمو ~16% سنوياً حسب IMARC، وتقديرات أخرى أقل بكثير ([IMARC](https://www.imarcgroup.com/saudi-arabia-cctv-market)).
4. **لا يوجد منافس يقدّم الامتثال السعودي كمنتج.** الأخطر: Eagle Eye (مركز بيانات في الرياض 2024) وRIME السعودية (6,000+ كاميرا، 2,000+ موقع، تمويل بذري أغسطس 2026) ([ISJ](https://internationalsecurityjournal.com/eagle-eye-networks-data-centre-2/)، [Bridge](https://bridgemena.com/news/saudi-based-rime-secures-over-2-million-seed-round-to-advance-ai-powered-enterprise-surveillance-solutions)).
5. **التقنية صالحة:** Frigate (MIT، آخر مستقر 0.17.2، و0.18.0-rc1) وgo2rtc (MIT، v1.9.14) يوفران كل ما نحتاجه عبر MQTT وHTTP، لكن نموذجهما الأمني منزلي ويستلزم عزلاً صارماً ([Frigate](https://github.com/blakeblackshear/frigate/releases)، [go2rtc](https://github.com/AlexxIT/go2rtc/releases)).

## حقائق تقنية أساسية
- MQTT: `frigate/events` (new/update/end مع before/after)، `frigate/reviews` (severity alert/detection)، `frigate/available`، `frigate/tracked_object_update` ([Docs](https://docs.frigate.video/integrations/mqtt)).
- HTTP: `/api/events`، `/api/events/<id>/{thumbnail.jpg,snapshot.jpg,clip.mp4}`، `/api/events/<id>/retain` (قفل الحوادث)، `/api/<camera>/latest.jpg`، `/api/events/search` (بحث دلالي) ([event.py](https://raw.githubusercontent.com/blakeblackshear/frigate/dev/frigate/api/event.py)).
- المنفذ 5000 في Frigate بلا مصادقة (داخلي فقط)، و8971 بمصادقة JWT ([Auth](https://docs.frigate.video/configuration/authentication)).
- WebRTC في go2rtc: `POST /api/webrtc?src=` (WHEP) ([OpenAPI](https://raw.githubusercontent.com/AlexxIT/go2rtc/master/website/api/openapi.yaml)).
- السحابة: GCP Dammam متاحة (عبر CNTXT فقط)، Oracle جدة/الرياض متاحة، Azure Saudi East نوفمبر 2026، AWS "على المسار" لديسمبر 2026 ([Google](https://docs.cloud.google.com/docs/dammam-region-access)، [Microsoft](https://news.microsoft.com/source/emea/2026/08/microsoft-announces-saudi-arabia-east-datacenter-region-will-be-available-in-november-2026/)، [AWS](https://www.aboutamazon.com/news/aws/aws-cloud-region-saudi-arabia)).
- عتاد: N100 يعطي ~15–30ms للاستدلال، وHailo-8 ~7ms ([Frigate hardware](https://docs.frigate.video/frigate/hardware))؛ Hailo-8 بـ179.99$ ([Waveshare](https://www.waveshare.com/hailo-8.htm?sku=27841)).

## أهم 5 قرارات / مخاطر

| # | القرار أو الخطر | التوصية |
|---|---|---|
| 1 | **خطر تنظيمي: ربط الصندوق بالسحابة.** م3/2 من اللائحة: ربط وسائط التخزين بشبكات أخرى يتطلب متطلبات NCA **وموافقة الوزارة** ([أم القرى](https://www.uqn.gov.sa/details?p=23050)). وم5 من النظام: البيع والتركيب والتشغيل تتطلب موافقة مسبقة | رأي قانوني + تواصل مع المديرية العامة للأمن العام **قبل** التجربة؛ تصميم يعمل محلياً بالكامل لو انقطعت السحابة |
| 2 | **قرار: مدة الحفظ وحجم القرص.** 90 يوماً × 8 كاميرات بدقة كاملة مستمرة ≈ 31TB (حسابنا) | الحصول على وثيقة الشروط الرسمية فوراً؛ حاسبة حفظ في المنتج؛ تصميم صناديق بأقراص أكبر أو تسجيل مختلط |
| 3 | **قرار: القناة = المركّبون المرخصون.** شهادة الاستيفاء والتركيب المرخص يجعلانهم نقطة البيع الحقيقية | برنامج شركاء بعمولة متكررة + بوابة شريك قبل الإطلاق |
| 4 | **خطر أمني: Frigate/go2rtc.** ثغرة عالية في Frigate ≤0.17.2 مُصلحة في 0.18.0-rc1 فقط؛ go2rtc API يمنح تنفيذ أوامر إن كُشف ([Advisory](https://github.com/blakeblackshear/frigate/security/advisories/GHSA-74fj-g666-3rv9)، [go2rtc](https://github.com/AlexxIT/go2rtc)) | لا منافذ مكشوفة إطلاقاً، تعطيل exec/echo، تحديثات OTA موقعة، اعتماد 0.18.x، متابعة أسبوعية |
| 5 | **قرار: الصوت والوجوه معطلة افتراضياً.** الصوت والحراري يتطلبان موافقة المديرية (م8)، والتحليل المحدد للهوية ممنوع دون موافقة (م10)، والغرامات PDPL حتى 5 ملايين ريال ([DLA Piper](https://www.dlapiperdataprotection.com/index.html?c=SA&t=law)) | MVP بكشف كائنات فقط؛ استضافة سعودية كاملة؛ DPA وDPO وإجراء إخطار 72 ساعة |

## التسعير المقترح (افتراض)
15 / 25 / 35 ريال لكل كاميرا شهرياً (أساسي/ذكي/سلاسل)، مقابل Verkada ≈ 69–78 ريال + عتاد باهظ ([Safe and Sound](https://getsafeandsound.com/blog/verkada-cameras-price/)). صندوق مجاني مقابل التزام 24–36 شهراً. أول 10 عملاء: سلاسل مطاعم، محطات وقود، مستودع، فندق، مجمع سكني، محلات مستقلة، ومركّب شريك — في الرياض أولاً.

## الخطوات التالية
1. الحصول على **وثيقة الشروط** والملحق (1) الرسميين واستخراج المدة والمواصفات.
2. جلسة استشارة قانونية (نظام الكاميرات + PDPL) حول الربط السحابي وموافقة م5.
3. عروض أسعار عتاد محلية 2026 (صندوق + أقراص مراقبة + UPS).
4. اختبار ميداني لعدد الكاميرات الممكنة على N100 مع/بدون Hailo-8.
5. توقيع أول 3 مركّبين شركاء.
