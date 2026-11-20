-- 0010 隐患附件区分“上报时”与“整改后”的照片。

alter table issue_attachment add column if not exists kind text not null default 'report';
