-- 0030 建设方案六（6）/四：制度宣贯「按岗位精准推送到岗」。
-- 原来只能按部门（dept_ids）下发，这里补岗位维度；两个都为空表示全员。

alter table broadcast_task add column if not exists post_ids text[] not null default '{}';

create index if not exists broadcast_task_post_idx on broadcast_task using gin (post_ids);
