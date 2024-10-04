package pkg

type ITopologicalSorter interface {
	Index() []interface{}
	Count() int
	InDegreeIsZero(index interface{}) bool
	InDegreeSubOne(index interface{}) error
	Children(index interface{}) []interface{}
	AddElement(index interface{})
}

// TopologicalSort returns a topological sorted ITopologicalSorter
// 不支持孤儿节点
func TopologicalSort(sorter ITopologicalSorter) (ITopologicalSorter, error) {
	res := make([]interface{}, 0)
	stack := &Stack{}
	idxSlice := sorter.Index()
	for _, idx := range idxSlice {
		if sorter.InDegreeIsZero(idx) {
			stack.Push(idx)
		}
	}
	if stack.IsEmpty() {
		return nil, ErrTypeNotDAG
	}
	for !stack.IsEmpty() {
		size := stack.Size()
		tmp := make([]interface{}, 0)
		for i := 0; i < size; i++ {
			top := stack.Pop()
			res = append(res, top)
			for _, childIndex := range sorter.Children(top) {
				if err := sorter.InDegreeSubOne(childIndex); err != nil {
					return nil, err
				}
				if sorter.InDegreeIsZero(childIndex) {
					tmp = append(tmp, childIndex)
				}
			}
		}
		for _, idx := range tmp {
			stack.Push(idx)
		}
	}
	if len(res) != sorter.Count() {
		return nil, ErrTypeNotDAG
	}
	for _, idx := range res {
		sorter.AddElement(idx)
	}

	return sorter, nil
}
