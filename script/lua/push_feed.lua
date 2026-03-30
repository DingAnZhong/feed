-- ==========================================================
-- 脚本功能：将帖子 Push 到用户的收件箱 (ZSet)，并限制收件箱最大长度
-- KEYS[1] : 用户的收件箱 Key，例如 "feed:timeline:2001"
-- ARGV[1] : 帖子的发布时间戳 (用作 ZSet 的 Score)
-- ARGV[2] : 帖子的 ID (用作 ZSet 的 Member)
-- ARGV[3] : 收件箱的最大容量限制 (例如 1000)
-- ==========================================================

local timeline_key = KEYS[1]
local post_score = ARGV[1]
local post_id = ARGV[2]
local max_length = tonumber(ARGV[3])

-- 1. 将帖子加入该用户的 ZSet 收件箱中
redis.call('ZADD', timeline_key, post_score, post_id)

-- 2. 获取当前收件箱的帖子总数
local current_length = redis.call('ZCARD', timeline_key)

-- 3. 如果超出了最大限制，则剔除最旧的帖子 (Score 最小的那些)
if current_length > max_length then
    -- ZREMRANGEBYRANK 删除排名在指定范围内的元素
    -- 0 表示分数最小的元素。截断多余的，保留倒数 max_length 个
    local remove_count = current_length - max_length
    redis.call('ZREMRANGEBYRANK', timeline_key, 0, remove_count - 1)
end

return 1