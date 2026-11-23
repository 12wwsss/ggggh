-- 0011 小程序答题记录（练题成绩，和正式考试分开）。

create table if not exists quiz_attempt (
    id         bigserial primary key,
    user_id    bigint references org_user(id) on delete cascade,
    job        text,
    score      int not null default 0,
    total      int not null default 0,
    wrong      jsonb not null default '[]'::jsonb,
    created_at timestamptz not null default now()
);

create index if not exists quiz_attempt_user_idx on quiz_attempt (user_id, id desc);
