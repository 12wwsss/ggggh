-- 0014 新员工带培计划。

create table if not exists training_plan (
    id         bigserial primary key,
    user_id    bigint not null references org_user(id) on delete cascade,
    title      text not null,
    mentor     text,
    start_date date,
    total_weeks int not null default 4,
    target     text,
    status     text not null default 'active' check (status in ('active','done','paused')),
    created_at timestamptz not null default now()
);

create table if not exists training_stage (
    id        bigserial primary key,
    plan_id   bigint not null references training_plan(id) on delete cascade,
    sort      int not null default 0,
    week      text,
    goal      text not null,
    tasks     jsonb not null default '[]'::jsonb,
    done      boolean not null default false,
    done_at   timestamptz
);

-- 示例：给新员工陈刚（worker04）一份带培计划
insert into training_plan (user_id, title, mentor, start_date, total_weeks, target, status)
select id, '新员工入矿带培计划', '李建国', current_date - 14, 4,
       '4 周内完成入矿教育、岗位跟班与独立上岗考核', 'active'
from org_user where username='worker04'
  and not exists (select 1 from training_plan where user_id=org_user.id);

insert into training_stage (plan_id, sort, week, goal, tasks, done, done_at)
select p.id, 1, '第 1 周', '入矿教育与安全规程', '["完成三级安全教育","学习《煤矿安全规程》总则与入井规定"]'::jsonb,
       true, now() - interval '12 days'
from training_plan p join org_user u on u.id=p.user_id where u.username='worker04'
  and not exists (select 1 from training_stage s where s.plan_id=p.id and s.sort=1);

insert into training_stage (plan_id, sort, week, goal, tasks, done, done_at)
select p.id, 2, '第 2 周', '岗位跟班学习', '["跟随师傅下井熟悉作业环境","认识本岗位设备与安全设施"]'::jsonb,
       true, now() - interval '8 days'
from training_plan p join org_user u on u.id=p.user_id where u.username='worker04'
  and not exists (select 1 from training_stage s where s.plan_id=p.id and s.sort=2);

insert into training_stage (plan_id, sort, week, goal, tasks, done)
select p.id, 3, '第 3 周', '实操训练', '["停送电与挂牌闭锁实操","临时支护实操"]'::jsonb, false
from training_plan p join org_user u on u.id=p.user_id where u.username='worker04'
  and not exists (select 1 from training_stage s where s.plan_id=p.id and s.sort=3);

insert into training_stage (plan_id, sort, week, goal, tasks, done)
select p.id, 4, '第 4 周', '考核与独立上岗', '["通过岗位安全考试","师傅评价合格后独立上岗"]'::jsonb, false
from training_plan p join org_user u on u.id=p.user_id where u.username='worker04'
  and not exists (select 1 from training_stage s where s.plan_id=p.id and s.sort=4);
