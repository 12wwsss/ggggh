-- 0031 建设方案六（1）：安全文化内容的后台运营（新增/编辑/排序/上下架）。

alter table content_item add column if not exists status text not null default 'published';
alter table content_item add column if not exists updated_at timestamptz not null default now();
alter table content_item add column if not exists updated_by bigint references org_user(id) on delete set null;

do $$
begin
    if not exists (
        select 1 from pg_constraint where conname = 'content_item_status_check'
    ) then
        alter table content_item add constraint content_item_status_check
            check (status in ('draft', 'published', 'archived'));
    end if;
end $$;

create index if not exists content_item_status_idx on content_item (kind, status, sort);

insert into role_policy (role, resource, action, scope) values
    ('admin',      'content', 'read',  'all'),
    ('admin',      'content', 'write', 'all'),
    ('safety',     'content', 'read',  'all'),
    ('safety',     'content', 'write', 'all'),
    ('dept_leader','content', 'read',  'all'),
    ('employee',   'content', 'read',  'all')
on conflict (role, resource, action) do update set scope = excluded.scope;
