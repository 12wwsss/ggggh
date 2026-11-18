-- 0007 内容模块：安全文化 / 图文 / 制度宣贯，供小程序内容页读取。
-- payload 直接存页面需要的对象，避免为每种内容建一张表。

create table if not exists content_item (
    id         text primary key,
    kind       text not null,            -- article | culture | notice
    cat        text,
    title      text not null default '',
    sort       int not null default 0,
    payload    jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now()
);

create index if not exists content_item_kind_idx on content_item (kind, cat, sort);
