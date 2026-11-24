-- 0019 站内消息。

create table if not exists message (
    id         bigserial primary key,
    user_id    bigint not null references org_user(id) on delete cascade,
    kind       text not null default 'system',
    title      text not null,
    body       text not null default '',
    ref_type   text,
    ref_id     bigint,
    read       boolean not null default false,
    created_at timestamptz not null default now()
);

create index if not exists message_user_idx on message (user_id, id desc);
create index if not exists message_unread_idx on message (user_id) where not read;
