-- 0004 微信小程序登录：openid 绑定与一次性登录票据

create table wx_bind (
    id            bigserial primary key,
    openid        text not null unique,
    unionid       text,
    user_id       bigint not null references org_user(id) on delete cascade,
    bound_at      timestamptz not null default now(),
    last_login_at timestamptz
);

create unique index wx_bind_user_idx on wx_bind(user_id);

-- 一次性票据：小程序登录后拿到短票据，用 web-view 打开 H5 时换会话，
-- 避免把 JWT 放在 URL 里被日志记录。
create table login_ticket (
    code       text primary key,
    user_id    bigint not null references org_user(id) on delete cascade,
    expires_at timestamptz not null,
    used_at    timestamptz,
    created_at timestamptz not null default now()
);

create index login_ticket_expiry_idx on login_ticket(expires_at);
