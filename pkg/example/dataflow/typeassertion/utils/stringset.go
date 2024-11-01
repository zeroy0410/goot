package utils

type StringSet map[string]struct{}

// Add 向集合中添加一个元素
func (s StringSet) Add(value string) {
	s[value] = struct{}{}
}

// Remove 从集合中移除一个元素
func (s StringSet) Remove(value string) {
	delete(s, value)
}

// Contains 检查集合中是否包含某个元素
func (s StringSet) Contains(value string) bool {
	_, exists := s[value]
	return exists
}

// Size 返回集合中元素的数量
func (s StringSet) Size() int {
	return len(s)
}