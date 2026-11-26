-- 0033 二期：问题带岗位（上报时取上报人岗位快照），报表可按岗位分布。

alter table issue add column if not exists post text not null default '';
create index if not exists issue_post_idx on issue(post);

-- 历史问题没有岗位，按上报人当前岗位回填一次（只补空值，不覆盖已有快照）。
update issue i
   set post = coalesce(u.post, '')
  from org_user u
 where u.id = i.reporter_id
   and coalesce(i.post, '') = '';
