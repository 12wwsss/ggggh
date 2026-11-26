-- 0025 岗位基础数据：把原来从账号里去重的岗位变成可维护的表。

create table if not exists org_post (
    id         bigserial primary key,
    name       text not null unique,
    sort       int not null default 0,
    status     text not null default 'active' check (status in ('active','disabled')),
    created_at timestamptz not null default now()
);

insert into org_post(name)
select distinct post from org_user where coalesce(post,'') <> ''
on conflict (name) do nothing;

insert into org_post(name)
select distinct post from learn_item where coalesce(post,'') <> ''
on conflict (name) do nothing;

insert into org_post(name) values ('通用') on conflict (name) do nothing;
