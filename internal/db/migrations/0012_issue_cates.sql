-- 0012 隐患分类：小程序上报的四个类型，供列表显示与筛选。

insert into issue_cate (name) values ('安全隐患'), ('设备故障'), ('制度疑问'), ('其他')
on conflict (name) do nothing;
