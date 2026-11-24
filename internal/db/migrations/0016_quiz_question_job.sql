-- 0016 题库支持岗位/分类与外部导入标识。

alter table quiz_question add column if not exists job text;
alter table quiz_question add column if not exists category text;
alter table quiz_question add column if not exists ext_id text;

create unique index if not exists quiz_question_ext_id_uidx on quiz_question (ext_id);
