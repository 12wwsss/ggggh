-- 0005 考试次数：试卷可设置允许多次考试，每次考试生成一条记录。

alter table exam_paper add column if not exists max_attempts int not null default 1;

alter table exam_record add column if not exists attempt_no int not null default 1;

-- 原来一人一卷只有一条记录，改为按次数允许多条
alter table exam_record drop constraint if exists exam_record_paper_id_user_id_key;
alter table exam_record drop constraint if exists exam_record_paper_id_user_id_attempt_no_key;
alter table exam_record add constraint exam_record_attempt_uniq unique (paper_id, user_id, attempt_no);

create index if not exists exam_record_user_idx on exam_record (paper_id, user_id, started_at desc);
