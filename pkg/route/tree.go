package route

import (
	"fmt"
	"github.com/Yuki-J1/wailsrouter/internal/bytesconv"
	"github.com/Yuki-J1/wailsrouter/pkg/app/server"
	"net/url"
	"strings"
)

type RadixTree struct {
	root *node
}

type (
	node struct {
		kind       kind
		label      byte
		prefix     string
		parent     *node
		children   children
		ppath      string
		pnames     []string
		handlers   server.HandlersChain
		paramChild *node
		anyChild   *node
		isLeaf     bool
	}
	kind     uint8
	children []*node
)

const (
	// static kind
	skind kind = iota
	// param kind
	pkind
	// all kind
	akind
	paramLabel = byte(':')
	anyLabel   = byte('*')
	slash      = "/"
	nilString  = ""
)

// checkPahtValid 对path进行检查，如果不符合规范，panic
func checkPahtValid(path string) {
	// path 不能为空
	if path == nilString {
		panic("empty path")
	}

	// path 必须以 '/' 开头
	if path[0] != '/' {
		panic("path must begin with '/'")
	}

	for i, c := range []byte(path) {
		switch c {
		// eg1：/user/:name/ 允许
		// eg2：/user/:/ 不允许
		// eg3：/user/:	不允许

		// eg4：/user/* 不允许
		// eg5：/user*name/ 不允许
		// eg6：/user/*name/ 不允许
		case ':':
			// 那么在第6次循环时，c == ':'，i == 5

			// :后面紧接着/ 或 只有: 都会panic
			if (i < len(path)-1 && path[i+1] == '/') /* :后面是/ */ || /* 或 */ i == (len(path)-1) /* 只有: */ {
				panic("wildcards must be named with a non-empty name in path '" + path + "'")
			}
			i++
			// 分隔符之间只能存在一个 : 或 * 否则 panic
			for ; i < len(path) && path[i] != '/'; i++ {
				if path[i] == ':' || path[i] == '*' {
					panic("only one wildcard per path segment is allowed, find multi in path '" + path + "'")
				}
			}
		case '*':
			// * 之后必须存在
			// /user/* 引发panic
			if i == len(path)-1 {
				panic("wildcards must be named with a non-empty name in path '" + path + "'")
			}
			// * 之前必须是 /
			// /user*name/ 引发panic
			if i > 0 && path[i-1] != '/' {
				panic(" no / before wildcards in path " + path)
			}
			// * 之后必须没有 /
			// /user/*name/ 引发panic
			for ; i < len(path); i++ {
				if path[i] == '/' {
					panic("catch-all routes are only allowed at the end of the path in path '" + path + "'")
				}
			}
		}

	}
}

func (r *RadixTree) addRoute(path string, h server.HandlersChain) {
	// 对path进行检查，如果不符合规范，panic
	checkPahtValid(path)

	var (
		pnames []string // Param names
		ppath  = path   // Pristine path
	)

	// 路由规则对应的处理函数不能为空
	if h == nil {
		panic(fmt.Sprintf("Adding route without handler function: %v", path))
	}

	for i /* 递增指针 */, lcpIndex /* path长度 */ := 0, len(path); i < lcpIndex; i++ {
		// 第一种情况：在path中当遇到:
		// 假设 /user/:name 此时i = 6
		if path[i] == paramLabel {
			// j 表示:后面的字符位置
			j := i + 1
			// 先插入:前面的部分 /user/
			r.insert(path[:i], nil, skind, nilString, nil)
			// 将i指向下一个 / 字符 或 i指向path结尾
			for ; i < lcpIndex && path[i] != '/'; i++ {
			}
			// 将参数name添加到参数列表
			// path[j:i] 表示:到/之间后面参数部分
			pnames = append(pnames, path[j:i])

			// path 变成没有参数 /user/:name > /user/:
			path = path[:j] + path[i:]

			// i path中指向:后面的字符的下标
			// lcpIndex 变更后的path长度
			i, lcpIndex = j, len(path)

			// 说明原始插入字符串的结尾没有/
			if i == lcpIndex {
				// 插入 /user/:
				r.insert(path[:i], h, pkind, nilString, pnames)
				return
			} else
			// 说明原始插入字符串的结尾有/
			{
				// 插入 /user/: 但没有 h
				r.insert(path[:i], nil, pkind, nilString, pnames)
			}
		} else
		// 第二种情况：在path中当遇到*
		// 假设 /user/*name 此时i = 6
		if path[i] == anyLabel {
			// 插入 /user/ 无h
			r.insert(path[:i], nil, skind, nilString, nil)
			// 将参数 name 添加到参数列表
			pnames = append(pnames, path[i+1:])
			// 插入 /user/* 有h
			r.insert(path[:i+1], h, akind, ppath, pnames)
			return
		}
	}
	// 第三种情况 : 插入的path是静态路由
	r.insert(path, h, skind, ppath, pnames)
}

func (r *RadixTree) insert(path string, h server.HandlersChain, t kind, ppath string, pnames []string) {
	// currentNode 指向根节点
	currentNode := r.root
	// currentNode 为nil panic
	if currentNode == nil {
		panic("invalid node")
	}
	// path 是要插入的字符串原始样子
	// search 是要被切割的字符串
	search := path

	// https://i.miji.bid/2023/11/26/361e4cd59e1419909a7fe79a0b364110.png
	for {
		// searchLen 是要被切割的字符串的长度
		searchLen := len(search)
		// prefoexLen 是currentNode所指的节点前缀的长度
		prefixLen := len(currentNode.prefix)
		// lcpLen (longest common prefix) 代表公共部分确切长度
		lcpLen := 0
		// max 代表公共部分可能的最大长度 (prefixLen 和 searchLen 的最小值)
		max := prefixLen
		if searchLen < max {
			max = searchLen
		}
		// 通过search和currentNode.prefix的前缀对比，找到公共部分确切长度(lcpLen)
		for ; lcpLen < max && search[lcpLen] == currentNode.prefix[lcpLen]; lcpLen++ {
		}

		// 如果公共部分确切长度(lcpLen)为0 (即没有公共部分)
		// 比如：第一次插入 search = "/user"  currentNode.prefix = "" 这时候 lcpLen(公共部分长度) == 0
		if lcpLen == 0 {
			// 在根节点，相当于给根节点更新一下
			// 为currentNode所指节点添加边信息(这个节点的前缀首字母) a
			currentNode.label = search[0]
			// 为currentNode所指节点添加前缀信息 ab
			currentNode.prefix = search
			// 当处理函数不为空时，为currentNode所指节点添加 类型、处理函数、路径、参数信息
			if h != nil {
				currentNode.kind = t
				currentNode.handlers = h
				currentNode.ppath = ppath
				currentNode.pnames = pnames
			}
			// 当currentNode所指的节点 无子节点 无:节点 无*节点
			// currentNode所指的节点才是叶子节点
			currentNode.isLeaf = currentNode.children == nil && currentNode.paramChild == nil && currentNode.anyChild == nil
		} else
		// 当lcpLen(公共部分长度) 小于 currentNode所指的节点的前缀长度
		if lcpLen < prefixLen {

			// region ========== 分裂第一步 创建currentNode所指节点的副本 ==========

			// 1. 副本和currentNode所指节点区别在于prefix其他不变
			// 2. 副本的父节点是 currentNode所指节点
			n := newNode(
				currentNode.kind,
				currentNode.prefix[lcpLen:],
				currentNode,
				currentNode.children,
				currentNode.handlers,
				currentNode.ppath,
				currentNode.pnames,
				currentNode.paramChild,
				currentNode.anyChild,
			)

			// endregion

			// region ========== 分裂第二步 将currentNode所指节点的孩子父亲改为n(currentNode所指节点的副本) (子节点转移) (currentNode所指节点减去一层那么所有孩子也都减去一层)  ==========
			for _, child := range currentNode.children {
				child.parent = n
			}
			if currentNode.paramChild != nil {
				currentNode.paramChild.parent = n
			}
			if currentNode.anyChild != nil {
				currentNode.anyChild.parent = n
			}
			// endregion

			// region ========== 分裂第三步 更新currentNode所指节点(可能被特殊情况重置) ==========

			currentNode.kind = skind
			currentNode.label = currentNode.prefix[0]
			currentNode.prefix = currentNode.prefix[:lcpLen]
			currentNode.children = nil
			currentNode.handlers = nil
			currentNode.ppath = nilString
			currentNode.pnames = nil
			currentNode.paramChild = nil
			currentNode.anyChild = nil
			currentNode.isLeaf = false

			// endregion

			// region ========== 分裂第四步 连接  ==========
			currentNode.children = append(currentNode.children, n)
			// endregion

			// 无论是那种情况只要 是符合条件：当lcpLen(公共部分长度) 小于 currentNode所指的节点的前缀长度
			// 肯定会先进行分裂，再根据具体情况执行不同逻辑
			if lcpLen == searchLen {
				// region ========== 插入特殊情况 会重置分裂第三步 ==========

				// 插入特殊情况 会重置分裂第三步
				// 特殊情况：search 是 currentNode.prefix 的子串
				// 需要更新currentNode所指节点(根据本次插入的 参数进行修改 保存handlers等等)
				// https://i.miji.bid/2023/11/26/17d067c8f0bd45315d9a157291b5324b.png
				currentNode.kind = t
				currentNode.handlers = h
				currentNode.ppath = ppath
				currentNode.pnames = pnames

				// endregion
			} else {
				// region ========== 插入情况：还有多出一个子节点，保存本次插入的handlers ==========

				// https://i.miji.bid/2023/11/26/38366484c8bddb3f01f14fc82b50a3a6.png
				n = newNode(t, search[lcpLen:], currentNode, nil, h, ppath, pnames, nil, nil)
				currentNode.children = append(currentNode.children, n)
				// endregion
			}
			// 判断currentNode所指节点是否为叶子节点
			currentNode.isLeaf = currentNode.children == nil && currentNode.paramChild == nil && currentNode.anyChild == nil
		} else
		// 当lcpLen(公共部分长度) 小于 search 长度
		// https://i.miji.bid/2023/11/26/695dcac0259f92521a317b3f94f53fc5.png
		if lcpLen < searchLen {
			// search 切去公共部分
			search = search[lcpLen:]
			// 查看currentNode所指节点 有没有以search[0]字符开头的子节点
			c := currentNode.findChildWithLabel(search[0])
			if c != nil {
				// 更新currentNode指针的指向(指针指向查询到的子节点)
				// 继续循环
				currentNode = c
				continue
			}

			// 如果没有以search[0]字符开头的子节点 则创建子节点保存此次插入的参数
			n := newNode(t, search, currentNode, nil, h, ppath, pnames, nil, nil)
			// 根据类型将创建的子节点 和 currentNode所指节点 形成关系(append)
			switch t {
			case skind:
				currentNode.children = append(currentNode.children, n)
			case pkind:
				currentNode.paramChild = n
			case akind:
				currentNode.anyChild = n
			}
			currentNode.isLeaf = currentNode.children == nil && currentNode.paramChild == nil && currentNode.anyChild == nil
		} else {
			if currentNode.handlers != nil && h != nil {
				panic("handlers are already registered for path '" + ppath + "'")
			}

			if h != nil {
				currentNode.handlers = h
				currentNode.ppath = ppath
				currentNode.pnames = pnames
			}
		}
		return
	}

}

func newNode(t kind, pre string, p *node, child children, mh server.HandlersChain, ppath string, pnames []string, paramChildren, anyChildren *node) *node {
	return &node{
		kind:       t,
		label:      pre[0],
		prefix:     pre,
		parent:     p,
		children:   child,
		ppath:      ppath,
		pnames:     pnames,
		handlers:   mh,
		paramChild: paramChildren,
		anyChild:   anyChildren,
		isLeaf:     child == nil && paramChildren == nil && anyChildren == nil,
	}
}

// --------------------------------------------------------------------------------------------------------

type nodeValue struct {
	handlers server.HandlersChain
	tsr      bool
	fullPath string
}

// find 通过路径查找已注册的处理程序，解析 URL 参数并将参数放入上下文中
func (r *RadixTree) find(path string, paramsPointer *server.Params, unescape bool) (res nodeValue) {
	var (
		cn          = r.root // cn 初始指向根
		search      = path   // path 是要查询字符串原始样子 search 是要被切割的字符串
		searchIndex = 0      // search 上的索引 可变
		buf         []byte
		paramIndex  int // 表示 paramsPointer中已有的参数个数 可变
	)

	// region ========== 回溯：向上退出一层、根据类型优先级找到下个节点、重置状态 指针 方便继续搜索 ==========

	backtrackToNextNodeKind := func(fromKind kind) (nextNodeKind kind, vaild bool) {
		previous := cn       // previous 和 cn 指向同一个节点设为X
		cn = previous.parent // cn 更新指向 X的父节点
		// valid 表示是否回退回溯成功
		vaild = cn != nil

		// 根据kind优先级确定回退回溯到那种类型
		// 任意节点之后,查找静态节点(skind)
		// 静态节点之后,查找参数节点(pkind)
		// 参数节点之后,查找任意节点(akind)
		if previous.kind == akind {
			nextNodeKind = skind
		} else {
			nextNodeKind = previous.kind + 1
		}
		if fromKind == skind {
			return
		}

		if previous.kind == skind {
			// previous 所指的X节点 为静态节点
			// searchIndex 指针向前回溯的个数是：len(previous.prefix)
			searchIndex -= len(previous.prefix)
		} else {
			// previous 所指的X节点 为:节点或者*节点
			// 参数指针-1
			paramIndex--
			// 对于:或者*节点 , prefix的值是: 或者 *, 因此无法从prefix中知道searchIndex应该向前回溯多少。
			// 但实际的参数值被保存到参数列表中，根据参数索引获得参数值。参数值就是searchIndex应该向前回溯的个数。
			searchIndex -= len((*paramsPointer)[paramIndex].Value)
			// 从paramsPointer中删除最后一个参数
			(*paramsPointer) = (*paramsPointer)[:paramIndex]
		}
		search = path[searchIndex:]
		return
	}

	// endregion

	// region ========== 顺序: static > param > any ==========
	for {
		// region ========== 静态 ==========

		// 判断当前节点是否是静态节点
		if cn.kind == skind {
			if len(search) >= len(cn.prefix) && cn.prefix == search[:len(cn.prefix)] {
				// region ========== 会被切割的字符串长度 >= cn指向的节点prefix长度 并且 cn指向的节点prefix是切割后的字符串子串/相对 ==========

				// eg1：cn指向的节点prefix是 /api/v1 会被切割的字符串是 /api/v1/users
				// eg2：cn指向的节点prefix是 /api/v1 会被切割的字符串是 /api/v1
				// eg3：cn指向的节点prefix是 /api/v1 会被切割的字符串是 /api/v1/
				// 切割search，去除共同前缀
				// eg1：search = /users
				// eg2: search = ""
				// eg3: search = /
				search = search[len(cn.prefix):]
				// searchIndex 指针在search上向前移动len(cn.prefix)
				// 7
				searchIndex = searchIndex + len(cn.prefix)

				// endregion
			} else {
				// region ========== 会被切割的字符串长度 < cn指向的节点prefix长度 或 cn指向的节点prefix不是切割后的字符串子串/相对 ==========

				// region ========== 重定向 ==========

				//	1. cn指向的节点prefix长度比会被切割的字符串长度大1
				//	2. cn指向的节点prefix最后一个字符是 /
				//	3. 会被切割的字符串 是 cn指向的节点prefix的子串
				// 	4. 当前节点有处理函数 或 当前节点有子节点
				// eg:  cn指向的节点prefix是 /xx/ 会被切割的字符串是 /xx
				// eg1: cn指向的节点prefix是 /api/v1/ 会被切割的字符串是 /api/v1
				// eg2: cn指向的节点prefix是 /AB/ 会被切割的字符串是  /AB
				if (len(cn.prefix) == len(search)+1) &&
					(cn.prefix[len(search)]) == '/' &&
					cn.prefix[:len(search)] == search &&
					(cn.handlers != nil || cn.anyChild != nil) {
					res.tsr = true
				}

				// endregion

				// region ========== 没有匹配 回溯 ==========

				// No matching prefix, let's backtrack to the first possible alternative node of the decision path
				nk, ok := backtrackToNextNodeKind(skind)
				// 回溯失败
				if !ok {
					return // No other possibilities on the decision path
				} else
				// 回溯 选择的节点是param类型，goto到对应逻辑
				if nk == pkind {
					goto Param
				} else
				// 其他情况
				// 这对于我们当前正在寻找的静态节点来说应该是不可能的
				{
					// Not found (this should never be possible for static node we are looking currently)
					break
				}
				// endregion

				// endregion
			}
		}

		// region ========== search 切去共同部分后, search的值为""(切完了) 并且  cn指向的节点有handlers。 说明找到了 ==========
		if search == nilString && len(cn.handlers) != 0 {
			res.handlers = cn.handlers
			break
		}
		// endregion

		// region ========== search 切去共同部分后, search的值不为""(没切完) ==========
		if search != nilString {
			// region ========== 重定向 ==========

			// 没切完 search切到就剩 / 的时候。并且 cn指向的节点有handlers。
			// 这时我们应该将tsr 设置为 true(重定向)
			// 例如：search 原本是  /api/v1/ ，cn指向的节点的prefix 是 /api/v1，search切去共同部分的时候还剩 / 并且 cn指向的节点有handlers。
			if search == "/" && cn.handlers != nil {
				res.tsr = true
			}
			// endregion

			// region ========== 继续切割search，向下找 ==========

			// 检查cn指向的节点是否有search[0]字符开头的子节点
			if child := cn.findChild(search[0]); child != nil {
				// cn指针更新 指向以search[0]字符开头的子节点
				cn = child
				// 继续查找 切search
				continue
			}

			// endregion
		}
		// endregion

		// region ========== search 切去共同部分后, search的值为""(切完了)  =========

		// search切完了 同时 cn指向的节点没有handlers，这时可能需要重定向
		// 如果cn指向的节点有以 / 开头的子节点，同时 子节点有handlers，重定向
		if search == nilString {
			if cd := cn.findChild('/'); cd != nil /* 如果cn指向的节点有以 / 开头的子节点 */ &&
				(cd.handlers != nil || cd.anyChild != nil) /* 并且 子节点有handlers或者 xxx  */ {
				res.tsr = true
			}
		}

		// endregion

		// endregion

		// region ========== Param ==========
	Param:
		// cn所指节点的paramChild不为nil 并且 search会被切割的字符串!=""。这时说明 cn所指节点是Param节点
		if child := cn.paramChild; search != nilString && child != nil {
			cn = child
			// i指向search中 / 字符所在的下标，也就是参数值结束位置。
			i := strings.Index(search, slash)
			if i == -1 {
				// 说明search 中没有 / 字符
				// i指向search最后尾部
				i = len(search)
			}
			// 参数列表容量扩大1个= 表示参数列表中已有的参数个数 + 扩1
			(*paramsPointer) = (*paramsPointer)[:(paramIndex + 1)]
			// 取出param的值
			val := search[:i]
			// 如果需要对参数值进行反转义, 则执行反转义操作
			if unescape {
				if v, err := url.QueryUnescape(search[:i]); err == nil {
					val = v
				}
			}
			// 将param值添加到参数列表
			(*paramsPointer)[paramIndex].Value = val
			// 现在已经向参数列表添加1个参数
			// paramIndex++表示参数列表中已有的参数个数 + 1
			paramIndex++

			// search切去/字符之前部分
			search = search[i:]
			searchIndex = searchIndex + i

			// search切完了
			if search == nilString {
				//	如果cn指向的节点有以 / 开头的子节点，同时 子节点有handlers，重定向
				if cd := cn.findChild('/'); cd != nil && (cd.handlers != nil || cd.anyChild != nil) {
					res.tsr = true
				}
			}
			continue
		}
		// endregion

		// region ========== Any ==========
	Any:
		// cn所指节点的anyChild不为nil
		if child := cn.anyChild; child != nil {
			cn = child
			// 参数列表容量扩大1个= 表示参数列表中已有的参数个数 + 扩1
			(*paramsPointer) = (*paramsPointer)[:(paramIndex + 1)]
			index := len(cn.pnames) - 1
			val := search
			// 如果需要对参数值进行反转义, 则执行反转义操作
			if unescape {
				if v, err := url.QueryUnescape(search); err == nil {
					val = v
				}
			}
			// 将param值添加到参数列表
			(*paramsPointer)[index].Value = bytesconv.B2s(append(buf, val...))
			// 更新索引 以备在找不到匹配的处理程序时进行回溯
			paramIndex++
			searchIndex += len(search)
			search = nilString
			res.handlers = cn.handlers
			break
		}
		// endregion

		// region ========== 没有找到匹配的处理程序 回溯 ==========

		// Let's backtrack to the first possible alternative node of the decision path
		// 没有匹配的前缀，让我们回溯到决策路径上的第一个可能的替代节点
		nk, ok := backtrackToNextNodeKind(akind)
		if !ok {
			break // No other possibilities on the decision path 决策路径上没有其他可能性
		} else if nk == pkind {
			goto Param
		} else if nk == akind {
			goto Any
		} else {
			// Not found
			break
		}

		// endregion
	}
	// endregion

	// region ========== 给参数赋值 ==========
	if cn != nil {
		res.fullPath = cn.ppath
		for i, name := range cn.pnames {
			(*paramsPointer)[i].Key = name
		}
	}
	// endregion
	return
}

// findChild 查找子节点中，是否存在前缀以 参数 开头的子节点
func (n *node) findChild(l byte) *node {
	for _, c := range n.children {
		if c.label == l {
			return c
		}
	}
	return nil
}
func (n *node) findChildWithLabel(l byte) *node {
	for _, c := range n.children {
		if c.label == l {
			return c
		}
	}
	if l == paramLabel {
		return n.paramChild
	}
	if l == anyLabel {
		return n.anyChild
	}
	return nil
}
