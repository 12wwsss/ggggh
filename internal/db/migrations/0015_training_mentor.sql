-- 0015 带培师傅改为账号绑定，避免同名误判。

alter table training_plan add column if not exists mentor_user_id bigint references org_user(id) on delete set null;

update training_plan p
set mentor_user_id = (select min(u.id) from org_user u where u.name = p.mentor and u.status = 'active')
where p.mentor_user_id is null and coalesce(p.mentor,'') <> '';
