-- 0029 建设方案第一批：培训员角色、风险点典型案例字段。

-- 五、使用角色里的「业务执行层（安全员、培训员）」：安全科已有 safety，这里补培训员。
alter table org_user drop constraint if exists org_user_role_check;
alter table org_user add constraint org_user_role_check
    check (role in ('admin', 'dept_leader', 'safety', 'trainer', 'employee'));

insert into role_policy (role, resource, action, scope) values
    ('trainer', 'doc',       'read',  'all'),
    ('trainer', 'qa',        'read',  'all'),
    ('trainer', 'learn',     'read',  'all'),
    ('trainer', 'learn',     'write', 'all'),
    ('trainer', 'quiz',      'read',  'all'),
    ('trainer', 'quiz',      'write', 'all'),
    ('trainer', 'exam',      'read',  'all'),
    ('trainer', 'broadcast', 'read',  'all'),
    ('trainer', 'training',  'read',  'all'),
    ('trainer', 'training',  'write', 'all'),
    ('trainer', 'dashboard', 'read',  'all'),
    ('trainer', 'content',   'read',  'all'),
    ('trainer', 'content',   'write', 'all')
on conflict (role, resource, action) do update set scope = excluded.scope;

-- 六（3）：「平台针对每个风险点推送典型案例和处置要点」——原来只有 measures，没有案例。
alter table risk_point add column if not exists case_text text not null default '';
