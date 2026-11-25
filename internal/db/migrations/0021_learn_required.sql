-- 0021 学习条目支持“必修”，用于学习计划与学时达标。

alter table learn_item add column if not exists required boolean not null default false;
