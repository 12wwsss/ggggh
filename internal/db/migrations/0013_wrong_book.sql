-- 0013 错题本（服务端，换设备不丢）。

create table if not exists wrong_book (
    id          bigserial primary key,
    user_id     bigint not null references org_user(id) on delete cascade,
    question_id bigint not null references quiz_question(id) on delete cascade,
    times       int not null default 1,
    updated_at  timestamptz not null default now(),
    unique (user_id, question_id)
);

create index if not exists wrong_book_user_idx on wrong_book (user_id, updated_at desc);
