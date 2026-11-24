-- 0017 视频/资料库。

create table if not exists media_item (
    id           bigserial primary key,
    title        text not null,
    category     text not null default '操作视频',
    type         text not null default 'video' check (type in ('video','image','doc')),
    url          text not null,
    size_bytes   bigint not null default 0,
    duration_sec int not null default 0,
    status       text not null default 'published' check (status in ('published','hidden')),
    sort         int not null default 0,
    created_by   bigint references org_user(id) on delete set null,
    created_at   timestamptz not null default now()
);
