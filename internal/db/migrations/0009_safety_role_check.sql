-- 0009 员工表角色约束放行安全科。

alter table org_user drop constraint if exists org_user_role_check;
alter table org_user add constraint org_user_role_check
    check (role in ('admin', 'dept_leader', 'safety', 'employee'));
