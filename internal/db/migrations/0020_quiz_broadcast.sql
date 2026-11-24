-- 0020 答题记录关联宣贯任务，用于统计宣贯配套答题。

alter table quiz_attempt add column if not exists broadcast_id bigint references broadcast_task(id) on delete set null;
create index if not exists quiz_attempt_broadcast_idx on quiz_attempt (broadcast_id, id desc);
