-- 0003 文档版本快照：发布时保存一份切片快照，支持查看、对比与回滚。

create table doc_version (
    id          bigserial primary key,
    doc_id      bigint not null references doc(id) on delete cascade,
    version     int not null,
    title       text not null,
    chunk_count int not null,
    operator_id bigint references org_user(id) on delete set null,
    snapshot    jsonb not null,
    created_at  timestamptz not null default now(),
    unique (doc_id, version)
);

create index doc_version_doc_idx on doc_version(doc_id, version desc);
