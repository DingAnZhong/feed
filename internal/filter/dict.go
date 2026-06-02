package filter

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TrieNode Trie 树节点
type TrieNode struct {
	children map[rune]*TrieNode
	isEnd    bool
}

// NewTrieNode 创建新节点
func NewTrieNode() *TrieNode {
	return &TrieNode{
		children: make(map[rune]*TrieNode),
		isEnd:    false,
	}
}

// Trie 敏感词 Trie 树
type Trie struct {
	root *TrieNode
	mu   sync.RWMutex
}

// NewTrie 创建新的 Trie 树
func NewTrie() *Trie {
	return &Trie{
		root: NewTrieNode(),
	}
}

// Insert 插入敏感词
func (t *Trie) Insert(word string) {
	word = strings.ToLower(strings.TrimSpace(word))
	if word == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	node := t.root
	for _, char := range word {
		if node.children[char] == nil {
			node.children[char] = NewTrieNode()
		}
		node = node.children[char]
	}
	node.isEnd = true
}

// BuildFromDictFile 从文件加载敏感词库
func (t *Trie) BuildFromDictFile(dictPath string) error {
	file, err := os.Open(dictPath)
	if err != nil {
		return err
	}
	defer file.Close()

	t.mu.Lock()
	defer t.mu.Unlock()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		if word == "" {
			continue
		}
		// 忽略以 # 开头的注释
		if strings.HasPrefix(word, "#") {
			continue
		}

		node := t.root
		for _, char := range word {
			if node.children[char] == nil {
				node.children[char] = NewTrieNode()
			}
			node = node.children[char]
		}
		node.isEnd = true
	}

	return scanner.Err()
}

// Search 搜索内容中的敏感词，返回第一个命中的敏感词，如果没有则返回空字符串
func (t *Trie) Search(content string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	content = strings.ToLower(content)

	for i := 0; i < len(content); i++ {
		node := t.root
		for j := i; j < len(content); j++ {
			char := rune(content[j])
			if node.children[char] == nil {
				break
			}
			node = node.children[char]
			if node.isEnd {
				return content[i : j+1]
			}
		}
	}

	return ""
}

// HasSensitiveWord 检查内容是否包含敏感词
func (t *Trie) HasSensitiveWord(content string) bool {
	return t.Search(content) != ""
}

// Replace 替换内容中的敏感词为 *
func (t *Trie) Replace(content string, replacement string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	content = strings.ToLower(content)
	var result strings.Builder
	result.Grow(len(content))

	for i := 0; i < len(content); i++ {
		node := t.root
		matchLength := 0

		for j := i; j < len(content); j++ {
			char := rune(content[j])
			if node.children[char] == nil {
				break
			}
			node = node.children[char]
			if node.isEnd {
				matchLength = j - i + 1
				break
			}
		}

		if matchLength > 0 {
			result.WriteString(replacement)
			i += matchLength - 1
		} else {
			result.WriteByte(content[i])
		}
	}

	return result.String()
}

// GlobalTrie 全局敏感词 Trie 树实例
var GlobalTrie *Trie

// InitDict 初始化敏感词库
func InitDict(dictPath string) error {
	// 如果没有提供路径，尝试从默认路径加载
	if dictPath == "" {
		// 尝试从当前目录的 dict 目录下加载
		defaultPaths := []string{
			"dict/sensitive_words.txt",
			"internal/filter/dict/sensitive_words.txt",
		}
		for _, path := range defaultPaths {
			if _, err := os.Stat(path); err == nil {
				dictPath = path
				break
			}
		}
	}

	GlobalTrie = NewTrie()

	// 如果提供了路径且文件存在，则加载
	if dictPath != "" {
		if _, err := os.Stat(dictPath); err == nil {
			return GlobalTrie.BuildFromDictFile(dictPath)
		}
	}

	// 如果没有加载到文件，使用默认的敏感词列表作为降级方案
	loadDefaultWords()
	return nil
}

// loadDefaultWords 加载默认敏感词（保底方案）
func loadDefaultWords() {
	defaultWords := []string{
		"违法", "赌博", "诈骗", "黄牛", "假币", "毒品", "枪支", "恐怖", "暴恐", "色情",
		"暴力", "血腥", "恐吓", "威胁", "侮辱", "诽谤", "谣言", "传销", "刷单", "返利",
		"套路贷", "高利贷", "爆炸", "炸药", "制造", "出售", "购买", "偷拍", "偷窥", "偷录",
		"窃听", "侵犯", "隐私", "间谍", "窃取", "国家", "政府", "领导人", "政治", "暴动",
		"起义", "颠覆", "反政府", "反党", "分裂", "独立", "台独", "港独", "维吾尔", "藏独",
		"法轮功", "全能神", "耶稣", "上帝", "邪教", "迷信", "算命", "看相", "驱鬼", "驱魔",
		"跳大神", "巫术", "喇嘛", "活佛", "转世", "菩萨", "佛祖", "释迦牟尼", "道教", "太上老君",
		"玉皇大帝", "天皇", "侵华", "南京", "大屠杀", "慰安妇", "细菌战", "731", "劳工", "强征",
		"性奴", "遗弃", "赔偿", "道歉", "靖国神社", "参拜", "否认", "美化", "侵略", "战争",
		"罪行", "受害者", "幸存者", "证人", "证据", "历史", "真相", "歪曲", "辩护", "辩解",
		"开脱", "洗地", "粉饰", "淡化", "回避", "抵赖", "狡辩", "诡辩", "歪理", "谬论",
		"谎言", "撒谎", "骗人", "欺骗", "欺诈", "坑蒙", "拐骗", "哄骗", "骗术", "陷阱",
		"圈套", "套路", "机关", "骗局", "假货", "伪劣", "假冒", "劣质", "有毒", "有害",
		"问题", "产品", "召回", "投诉", "曝光", "调查", "处罚", "罚款", "吊销", "执照",
		"关闭", "查封", "捣毁", "抓包", "网监", "网信", "网安", "网警",
	}
	for _, word := range defaultWords {
		GlobalTrie.Insert(word)
	}
}

// DetectSensitiveWord 检测内容中的敏感词（全局函数）
func DetectSensitiveWord(content string) string {
	if GlobalTrie == nil {
		return ""
	}
	return GlobalTrie.Search(content)
}

// HasSensitiveWords 检查内容是否包含敏感词（全局函数）
func HasSensitiveWords(content string) bool {
	if GlobalTrie == nil {
		return false
	}
	return GlobalTrie.HasSensitiveWord(content)
}

// ReplaceSensitiveWord 替换内容中的敏感词（全局函数）
func ReplaceSensitiveWord(content string, replacement string) string {
	if GlobalTrie == nil {
		return content
	}
	return GlobalTrie.Replace(content, replacement)
}

// GetDictFilePath 获取敏感词库文件路径
func GetDictFilePath() string {
	// 尝试从多个可能的路径查找
	paths := []string{
		"dict/sensitive_words.txt",
		"internal/filter/dict/sensitive_words.txt",
		"./dict/sensitive_words.txt",
		"../dict/sensitive_words.txt",
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			absPath, _ := filepath.Abs(path)
			return absPath
		}
	}
	return ""
}
