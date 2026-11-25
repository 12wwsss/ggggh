-- 0023 学习计划分配：按岗位/区队指派必修条目并设置截止日期。

create table if not exists learn_assign (
    id         bigserial primary key,
    item_id    bigint not null references learn_item(id) on delete cascade,
    post       text not null default '',
    dept_id    bigint references org_dept(id) on delete cascade,
    due_at     date,
    created_at timestamptz not null default now()
);

create index if not exists learn_assign_item_idx on learn_assign (item_id);
create index if not exists learn_assign_scope_idx on learn_assign (post, dept_id);
