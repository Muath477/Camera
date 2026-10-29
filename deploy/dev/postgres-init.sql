-- Login used by the services. It gets its table rights through camera_app,
-- which the migrations create; it is never a superuser, so row-level
-- security always applies.
CREATE ROLE camera_api LOGIN PASSWORD 'camera_api';
