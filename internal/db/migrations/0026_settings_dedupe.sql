-- 0026 系统配置 + 学习到期消息去重。

create table if not exists app_setting (
    key        text primary key,
    value      text not null default '',
    updated_at timestamptz not null default now()
);

insert into app_setting(key, value) values ('safety_contact', '') on conflict (key) do nothing;

alter table message add column if not exists dedupe_key text;
create unique index if not exists message_dedupe_uidx on message(user_id, dedupe_key) where dedupe_key is not null;
