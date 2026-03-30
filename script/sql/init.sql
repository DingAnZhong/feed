-- 创建数据库
CREATE DATABASE IF NOT EXISTS feed_db DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE feed_db;

-- 1. 用户表
CREATE TABLE IF NOT EXISTS `users` (
    `id` BIGINT NOT NULL COMMENT '用户ID (雪花算法生成)',
    `nickname` VARCHAR(64) NOT NULL COMMENT '昵称',
    `avatar` VARCHAR(255) DEFAULT '' COMMENT '头像URL',
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';

-- 2. 帖子表 (动态表)
CREATE TABLE IF NOT EXISTS `posts` (
    `id` BIGINT NOT NULL COMMENT '帖子ID (雪花算法生成)',
    `user_id` BIGINT NOT NULL COMMENT '作者的用户ID',
    `content` TEXT NOT NULL COMMENT '帖子文本内容',
    `media_urls` JSON COMMENT '图片/视频URL列表 (JSON格式)',
    `like_count` INT DEFAULT 0 COMMENT '点赞数',
    `comment_count` INT DEFAULT 0 COMMENT '评论数',
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user_id` (`user_id`) -- 经常需要查某个人的主页动态
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='帖子表';

-- 3. 用户关系表 (关注表)
CREATE TABLE IF NOT EXISTS `relations` (
    `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '自增主键 (仅做物理主键)',
    `follower_id` BIGINT NOT NULL COMMENT '关注者ID (粉丝)',
    `followee_id` BIGINT NOT NULL COMMENT '被关注者ID (大V)',
    `status` TINYINT DEFAULT 1 COMMENT '状态: 1-正常关注, 0-已取消',
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    -- 核心索引设计：
    UNIQUE KEY `uk_follower_followee` (`follower_id`, `followee_id`), -- 防止重复关注
    KEY `idx_followee_id` (`followee_id`) -- 核心：发帖时需要用这个索引极速查出大V的所有粉丝
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户关系表';



-- 1. 插入多层级的用户群
INSERT INTO `users` (`id`, `nickname`, `avatar`) VALUES 
-- 【超级大V】 (粉丝量巨大，适合测试 Pull 拉模式)
(1001, '明星大V_鹿晗', 'https://avatar.com/luhan.jpg'),
(1002, '明星大V_周杰伦', 'https://avatar.com/jay.jpg'),
(1003, '官方小助手', 'https://avatar.com/admin.jpg'),

-- 【中腰部KOL】 (垂直领域博主，适合测试 Push 推模式)
(1101, '科技数码区UP主', 'https://avatar.com/tech.jpg'),
(1102, '深夜放毒美食家', 'https://avatar.com/food.jpg'),

-- 【普通粉丝/吃瓜群众】
(2001, '普通粉丝_A (杰迷)', 'https://avatar.com/a.jpg'),
(2002, '普通粉丝_B (鹿饭)', 'https://avatar.com/b.jpg'),
(2003, '普通粉丝_C (吃货)', 'https://avatar.com/c.jpg'),
(2004, '普通粉丝_D (极客)', 'https://avatar.com/d.jpg'),
(2005, '普通粉丝_E (理智粉)', 'https://avatar.com/e.jpg');


-- 2. 建立错综复杂的关注网络
INSERT INTO `relations` (`follower_id`, `followee_id`, `status`) VALUES 
-- 场景 A：所有人都默认关注了【官方小助手】(1003)
(1001, 1003, 1), (1002, 1003, 1), (1101, 1003, 1), (1102, 1003, 1),
(2001, 1003, 1), (2002, 1003, 1), (2003, 1003, 1), (2004, 1003, 1), (2005, 1003, 1),

-- 场景 B：超级大V的粉丝群 (测试大V发帖时不把 Redis 瞬间写爆)
(2001, 1002, 1), -- 粉丝A关注周杰伦
(2005, 1002, 1), -- 粉丝E关注周杰伦
(2002, 1001, 1), -- 粉丝B关注鹿晗
(2005, 1001, 1), -- 粉丝E关注鹿晗 (双担粉)

-- 场景 C：垂直区博主的粉丝 (测试常规的写扩散/发件箱分发)
(2003, 1102, 1), -- 粉丝C关注美食家
(2004, 1101, 1), -- 粉丝D关注数码UP主
(2005, 1101, 1), -- 粉丝E关注数码UP主

-- 场景 D：普通用户的社交互关 (测试基础关注流)
(2001, 2002, 1), -- A关注B
(2002, 2001, 1), -- B回关A (互相关注)
(2003, 2004, 1), -- C关注D
(2004, 2003, 0); -- D关注了C，但后来又【取消关注】了 (status=0)