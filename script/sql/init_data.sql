USE feed_db;

INSERT INTO `users` (`id`, `nickname`, `avatar`) VALUES
(1001, 'StarLuHan', 'https://avatar.com/luhan.jpg'),
(1002, 'JayChou', 'https://avatar.com/jay.jpg'),
(1003, 'AdminBot', 'https://avatar.com/admin.jpg'),
(1101, 'TechUP', 'https://avatar.com/tech.jpg'),
(1102, 'FoodieMaster', 'https://avatar.com/food.jpg'),
(2001, 'FanA', 'https://avatar.com/a.jpg'),
(2002, 'FanB', 'https://avatar.com/b.jpg'),
(2003, 'FanC', 'https://avatar.com/c.jpg'),
(2004, 'FanD', 'https://avatar.com/d.jpg'),
(2005, 'FanE', 'https://avatar.com/e.jpg');

INSERT INTO `relations` (`follower_id`, `followee_id`, `status`) VALUES
(1001, 1003, 1), (1002, 1003, 1), (1101, 1003, 1), (1102, 1003, 1),
(2001, 1003, 1), (2002, 1003, 1), (2003, 1003, 1), (2004, 1003, 1), (2005, 1003, 1),
(2001, 1002, 1), (2005, 1002, 1),
(2002, 1001, 1), (2005, 1001, 1),
(2003, 1102, 1),
(2004, 1101, 1), (2005, 1101, 1),
(2001, 2002, 1), (2002, 2001, 1),
(2003, 2004, 1), (2004, 2003, 0);

-- 初始化帖子数据（status 默认为 0 - 正常）
INSERT INTO `posts` (`id`, `user_id`, `content`, `media_urls`, `status`) VALUES
(3001, 1001, '今天天气真好！#鹿晗 #明星日常', NULL, 0),
(3002, 1002, '新专辑录制中，大家期待吗？#周杰伦 #音乐', NULL, 0),
(3003, 1003, '欢迎来到 Feed 社交平台！', NULL, 0),
(3004, 1101, '最新款手机评测，科技爱好者必看！', NULL, 0),
(3005, 1102, '深夜放毒！这道菜绝了 #美食 #吃货', NULL, 0);
