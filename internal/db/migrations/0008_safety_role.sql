-- 0008 安全科角色：看全矿问题、参与受理与验收，但不管理用户和知识库。

insert into org_dept (name, code)
select '安全科', 'SAFETY' where not exists (select 1 from org_dept where name = '安全科');

insert into role_policy (role, resource, action, scope) values
    ('safety', 'issue',     'read',  'all'),
    ('safety', 'issue',     'write', 'all'),
    ('safety', 'dashboard', 'read',  'all'),
    ('safety', 'doc',       'read',  'all'),
    ('safety', 'learn',     'read',  'all'),
    ('safety', 'exam',      'read',  'all'),
    ('safety', 'broadcast', 'read',  'all'),
    ('safety', 'qa',        'read',  'all'),
    ('safety', 'quiz',      'read',  'all'),
    ('safety', 'user',      'read',  'all'),
    ('safety', 'dept',      'read',  'all')
on conflict (role, resource, action) do update set scope = excluded.scope;
