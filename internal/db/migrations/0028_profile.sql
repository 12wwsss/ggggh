-- 0028 个人资料：显示名与头像（不改在册姓名）。

alter table org_user add column if not exists display_name text;
alter table org_user add column if not exists avatar text;
