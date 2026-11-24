-- 0018 岗位风险点库。

create table if not exists risk_point (
    id          bigserial primary key,
    post        text not null default '通用',
    category    text not null default '通用',
    title       text not null,
    description text not null default '',
    measures    jsonb not null default '[]'::jsonb,
    level       text not null default '中' check (level in ('低','中','高','重大')),
    clause_refs jsonb not null default '[]'::jsonb,
    status      text not null default 'published' check (status in ('published','hidden')),
    sort        int not null default 0,
    created_at  timestamptz not null default now()
);

insert into risk_point (post, category, title, description, measures, level, clause_refs)
select * from (values
('采煤','顶板管理','煤壁片帮伤人','采煤机割煤后煤壁暴露，支护不及时易片帮。',
 '["割煤后及时伸出前探梁护顶","人员不得站在煤壁侧","发现片帮预兆立即撤人"]'::jsonb,'高','["第四十五条"]'::jsonb),
('采煤','顶板管理','单体支柱初撑力不足','支柱初撑力不够会导致顶板下沉、漏顶。',
 '["按规程升柱并二次注液","每班检查支柱压力","失效支柱立即更换"]'::jsonb,'高','["第五十二条"]'::jsonb),
('掘进','顶板管理','空顶作业','掘进后未及时支护就进入空顶区作业。',
 '["严禁空顶作业","临时支护及时跟进","敲帮问顶后再作业"]'::jsonb,'重大','["第四十八条"]'::jsonb),
('掘进','爆破管理','爆破警戒不到位','放炮前警戒距离不够或人员未撤离。',
 '["严格执行一炮三检","三人连锁放炮","警戒线设岗并清点人数"]'::jsonb,'重大','["第三百一十条"]'::jsonb),
('通风','瓦斯防治','局部通风机停风','停风后瓦斯积聚，盲目送电易引发爆炸。',
 '["停风立即撤人、断电、设栅栏","恢复前先检查瓦斯","风电闭锁必须有效"]'::jsonb,'重大','["第一百三十六条"]'::jsonb),
('通风','瓦斯防治','瓦斯超限作业','瓦斯浓度超限仍继续作业。',
 '["瓦斯达到 1.0% 停止电钻打眼","达到 1.5% 停止工作、断电撤人","报警后立即报告调度"]'::jsonb,'重大','["第一百三十六条"]'::jsonb),
('机电','设备检修','带电检修电气设备','未停电挂牌就检修，易触电。',
 '["停电、验电、放电、挂牌、闭锁","专人监护","检修后确认无人员再送电"]'::jsonb,'高','["第四百四十五条"]'::jsonb),
('机电','设备检修','电气设备失爆','接线腔密封损坏，产生电火花引爆瓦斯。',
 '["每班检查防爆面","失爆设备立即停用","严禁擅自拆改密封"]'::jsonb,'重大','["第四百五十二条"]'::jsonb),
('运输','运输管理','跨越运行中的带式输送机','人员跨越皮带易被卷入。',
 '["必须走过桥","严禁跨越或钻越皮带","皮带运行中禁止清理滚筒"]'::jsonb,'高','["第三百七十六条"]'::jsonb),
('通用','入井管理','入井前未检查装备','矿灯、安全帽、自救器缺失或失效。',
 '["入井前检查三件套","随身携带识别卡","酒后、疲劳不得入井"]'::jsonb,'高','["第六条"]'::jsonb)
) as v(post, category, title, description, measures, level, clause_refs)
where not exists (select 1 from risk_point);
