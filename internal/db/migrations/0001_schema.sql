-- 0001 初始表结构（一期 14 张 + 二期 4 张 + 任务表 + 元数据表）

create table org_dept (
    id          bigserial primary key,
    parent_id   bigint references org_dept(id) on delete set null,
    name        text not null,
    code        text unique,
    sort        int not null default 0,
    created_at  timestamptz not null default now()
);

create table org_user (
    id              bigserial primary key,
    dept_id         bigint references org_dept(id) on delete set null,
    username        text not null unique,
    password_hash   text not null,
    name            text not null,
    phone           text,
    post            text,
    role            text not null check (role in ('admin','dept_leader','employee')),
    status          text not null default 'active' check (status in ('active','disabled')),
    failed_attempts int not null default 0,
    locked_until    timestamptz,
    last_login_at   timestamptz,
    created_at      timestamptz not null default now(),
    updated_at      timestamptz not null default now()
);
create index org_user_dept_idx on org_user(dept_id);

create table role_policy (
    id        bigserial primary key,
    role      text not null,
    resource  text not null,
    action    text not null,
    scope     text not null default 'self' check (scope in ('all','dept','self')),
    unique (role, resource, action)
);

create table doc (
    id           bigserial primary key,
    title        text not null,
    category     text not null default '规程',
    source_name  text,
    stored_path  text,
    file_size    bigint not null default 0,
    sha256       text,
    status       text not null default 'parsing'
                 check (status in ('parsing','pending_review','published','rejected','failed')),
    version      int not null default 1,
    chunk_count  int not null default 0,
    uploader_id  bigint references org_user(id) on delete set null,
    error_msg    text,
    created_at   timestamptz not null default now(),
    updated_at   timestamptz not null default now(),
    published_at timestamptz
);
create index doc_status_idx on doc(status);

create table doc_chunk (
    id         bigserial primary key,
    doc_id     bigint not null references doc(id) on delete cascade,
    clause_no  text,
    chapter    text,
    seq        int not null,
    title      text,
    content    text not null,
    char_count int not null,
    part       int not null default 1,
    published  boolean not null default false,
    created_at timestamptz not null default now()
);
create index doc_chunk_doc_idx on doc_chunk(doc_id);
create index doc_chunk_clause_idx on doc_chunk(clause_no);

-- 向量持久化表：内存实现启动时从此表装载，pgvector 实现直接查此表。
create table doc_embedding (
    chunk_id   bigint primary key references doc_chunk(id) on delete cascade,
    model      text not null,
    dim        int not null,
    vec        real[] not null,
    updated_at timestamptz not null default now()
);

create table qa_log (
    id         bigserial primary key,
    user_id    bigint references org_user(id) on delete set null,
    dept_id    bigint,
    question   text not null,
    answer     text not null,
    citations  jsonb not null default '[]'::jsonb,
    answered   boolean not null default false,
    top_score  real not null default 0,
    hit_count  int not null default 0,
    latency_ms int not null default 0,
    model      text,
    feedback   text,
    ip         text,
    created_at timestamptz not null default now()
);
create index qa_log_user_idx on qa_log(user_id, created_at desc);
create index qa_log_dept_idx on qa_log(dept_id, created_at desc);

create table learn_item (
    id          bigserial primary key,
    title       text not null,
    category    text not null default '规程',
    post        text,
    dept_id     bigint references org_dept(id) on delete set null,
    content     text not null,
    clause_refs jsonb not null default '[]'::jsonb,
    doc_id      bigint references doc(id) on delete set null,
    sort        int not null default 0,
    status      text not null default 'published' check (status in ('draft','published','archived')),
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create table learn_record (
    id           bigserial primary key,
    item_id      bigint not null references learn_item(id) on delete cascade,
    user_id      bigint not null references org_user(id) on delete cascade,
    status       text not null default 'not_started'
                 check (status in ('not_started','in_progress','completed')),
    progress     int not null default 0 check (progress between 0 and 100),
    duration_sec int not null default 0,
    started_at   timestamptz,
    completed_at timestamptz,
    updated_at   timestamptz not null default now(),
    unique (item_id, user_id)
);

create table quiz_question (
    id         bigserial primary key,
    item_id    bigint references learn_item(id) on delete set null,
    stem       text not null,
    options    jsonb not null default '[]'::jsonb,
    answer     text not null,
    analysis   text,
    difficulty text not null default 'normal' check (difficulty in ('easy','normal','hard')),
    created_at timestamptz not null default now()
);
create index quiz_question_item_idx on quiz_question(item_id);

create table exam_paper (
    id           bigserial primary key,
    title        text not null,
    dept_id      bigint references org_dept(id) on delete set null,
    question_ids bigint[] not null default '{}',
    total_score  int not null default 100,
    pass_score   int not null default 60,
    duration_min int not null default 60,
    start_at     timestamptz,
    end_at       timestamptz,
    status       text not null default 'draft' check (status in ('draft','published','closed')),
    created_by   bigint references org_user(id) on delete set null,
    created_at   timestamptz not null default now()
);

create table exam_record (
    id          bigserial primary key,
    paper_id    bigint not null references exam_paper(id) on delete cascade,
    user_id     bigint not null references org_user(id) on delete cascade,
    answers     jsonb not null default '{}'::jsonb,
    score       int not null default 0,
    passed      boolean not null default false,
    status      text not null default 'in_progress'
                check (status in ('in_progress','submitted','timeout')),
    started_at  timestamptz not null default now(),
    submitted_at timestamptz,
    unique (paper_id, user_id)
);

create table broadcast_task (
    id           bigserial primary key,
    title        text not null,
    content      text not null,
    doc_ids      bigint[] not null default '{}',
    dept_ids     bigint[] not null default '{}',
    publisher_id bigint references org_user(id) on delete set null,
    require_ack  boolean not null default true,
    deadline     timestamptz,
    status       text not null default 'draft' check (status in ('draft','published','closed')),
    created_at   timestamptz not null default now()
);

create table broadcast_record (
    id       bigserial primary key,
    task_id  bigint not null references broadcast_task(id) on delete cascade,
    user_id  bigint not null references org_user(id) on delete cascade,
    status   text not null default 'pending' check (status in ('pending','read','ack')),
    read_at  timestamptz,
    ack_at   timestamptz,
    unique (task_id, user_id)
);

create table audit_log (
    id          bigserial primary key,
    user_id     bigint,
    username    text,
    action      text not null,
    resource    text not null,
    resource_id text,
    detail      jsonb not null default '{}'::jsonb,
    ip          text,
    ua          text,
    created_at  timestamptz not null default now()
);
create index audit_log_created_idx on audit_log(created_at desc);

-- 后台任务表：不引入 Redis，进程内 worker 轮询。
create table task (
    id           bigserial primary key,
    kind         text not null,
    payload      jsonb not null default '{}'::jsonb,
    status       text not null default 'pending'
                 check (status in ('pending','running','succeeded','failed')),
    attempts     int not null default 0,
    max_attempts int not null default 3,
    last_error   text,
    run_after    timestamptz not null default now(),
    locked_at    timestamptz,
    created_at   timestamptz not null default now(),
    updated_at   timestamptz not null default now()
);
create index task_poll_idx on task(status, run_after);

create table system_meta (
    key        text primary key,
    value      text not null,
    updated_at timestamptz not null default now()
);

-- ---------- 二期 ----------

create table issue_cate (
    id        bigserial primary key,
    name      text not null unique,
    parent_id bigint references issue_cate(id) on delete set null,
    sort      int not null default 0
);

create table issue (
    id             bigserial primary key,
    code           text not null unique,
    title          text not null,
    description    text not null default '',
    cate_id        bigint references issue_cate(id) on delete set null,
    dept_id        bigint references org_dept(id) on delete set null,
    reporter_id    bigint references org_user(id) on delete set null,
    assignee_id    bigint references org_user(id) on delete set null,
    severity       text not null default 'normal' check (severity in ('low','normal','high')),
    status         text not null default 'reported'
                   check (status in ('reported','accepted','rejected','rectified','verified','closed')),
    deadline       timestamptz,
    result         text,
    verify_comment text,
    overdue        boolean not null default false,
    created_at     timestamptz not null default now(),
    updated_at     timestamptz not null default now(),
    closed_at      timestamptz
);
create index issue_status_idx on issue(status);
create index issue_dept_idx on issue(dept_id);

-- 流转记录只插不改：代码中没有任何 UPDATE 该表的语句。
create table issue_flow (
    id          bigserial primary key,
    issue_id    bigint not null references issue(id) on delete cascade,
    action      text not null,
    from_status text,
    to_status   text not null,
    actor_id    bigint references org_user(id) on delete set null,
    comment     text,
    created_at  timestamptz not null default now()
);
create index issue_flow_issue_idx on issue_flow(issue_id, id);

create table issue_attachment (
    id          bigserial primary key,
    issue_id    bigint not null references issue(id) on delete cascade,
    file_name   text not null,
    stored_path text not null,
    file_size   bigint not null default 0,
    sha256      text,
    uploader_id bigint references org_user(id) on delete set null,
    created_at  timestamptz not null default now()
);
