-- 0035 问答转人工：员工在问答里点「转人工」，安全科在后台回复，员工在消息中心看到结果。

create table if not exists qa_ticket (
    id          bigserial primary key,
    log_id      bigint references qa_log(id) on delete set null,
    user_id     bigint references org_user(id) on delete set null,
    dept_id     bigint,
    question    text not null,
    answer      text not null default '',
    status      text not null default 'pending' check (status in ('pending', 'replied', 'closed')),
    assignee_id bigint references org_user(id) on delete set null,
    reply       text not null default '',
    replied_at  timestamptz,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create index if not exists qa_ticket_status_idx on qa_ticket (status, id desc);
create index if not exists qa_ticket_user_idx on qa_ticket (user_id, id desc);

-- 安全科和管理员能处理转人工；培训员只读
insert into role_policy (role, resource, action, scope) values
    ('safety', 'qa_ticket', 'read', 'all'),
    ('safety', 'qa_ticket', 'write', 'all'),
    ('admin', 'qa_ticket', 'read', 'all'),
    ('admin', 'qa_ticket', 'write', 'all'),
    ('trainer', 'qa_ticket', 'read', 'all')
on conflict (role, resource, action) do update set scope = excluded.scope;
