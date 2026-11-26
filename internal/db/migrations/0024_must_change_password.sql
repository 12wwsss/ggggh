-- 0024 首次登录强制修改初始口令。

alter table org_user add column if not exists must_change_password boolean not null default false;
