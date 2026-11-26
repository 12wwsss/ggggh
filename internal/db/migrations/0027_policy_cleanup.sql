-- 0027 清理历史上不合法的写入权限，和代码里的只读/管理员限制保持一致。

delete from role_policy where action='write' and resource in ('audit','dashboard','qa');
delete from role_policy where action='write' and resource in ('user','dept','task','eval') and role <> 'admin';
