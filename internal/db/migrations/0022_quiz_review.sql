-- 0022 题库审核状态。

alter table quiz_question add column if not exists status text not null default 'published';
alter table quiz_question drop constraint if exists quiz_question_status_check;
alter table quiz_question add constraint quiz_question_status_check check (status in ('pending','published','rejected'));
