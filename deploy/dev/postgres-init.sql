-- Login used by the services. It gets its table rights through camera_app;
-- it is never a superuser, so row-level security always applies.
-- camera_app is created here (the migration that grants it rights only
-- creates it when missing) so the login can be made a member before the
-- first migration runs. Without the membership every query is refused.
CREATE ROLE camera_app NOLOGIN;
CREATE ROLE camera_api LOGIN PASSWORD 'camera_api' IN ROLE camera_app;
