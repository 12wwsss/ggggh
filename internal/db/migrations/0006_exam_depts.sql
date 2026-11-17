-- 0006 试卷按部门发布：支持指定多个部门；空数组表示全员。

alter table exam_paper add column if not exists dept_ids bigint[] not null default '{}';

-- 把旧的单部门回填成数组，避免已有试卷变成“全员”
update exam_paper set dept_ids = array[dept_id] where dept_id is not null and cardinality(dept_ids) = 0;

create index if not exists exam_paper_depts_idx on exam_paper using gin (dept_ids);
