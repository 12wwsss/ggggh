-- 0034 带培师傅月度小结：师傅每月写一份徒弟表现小结，期满再写一份总结。
-- 按「网上的师带徒制度」：师傅需每月提交徒弟表现小结，协议期满提交培训总结。

create table if not exists training_note (
    id         bigserial primary key,
    plan_id    bigint not null references training_plan(id) on delete cascade,
    author_id  bigint references org_user(id) on delete set null,
    kind       text not null default 'monthly' check (kind in ('monthly', 'final')),
    period     text not null default '',
    content    text not null,
    created_at timestamptz not null default now()
);

create index if not exists training_note_plan_idx on training_note (plan_id, id desc);

-- 培训员要能看全矿带培、写小结、导培训档案；原来只给了 training 的读写，导出沿用 learn/exam 的读权限。
insert into role_policy (role, resource, action, scope) values
    ('trainer', 'learn', 'read', 'all')
on conflict (role, resource, action) do update set scope = excluded.scope;
