package client

import (
	"fmt"
	"net/http"
	"time"

	"api/pkg/database"
	"api/pkg/deepseek"
	"api/pkg/queue"
	pkgredis "api/pkg/redis"
	"api/pkg/response"

	"github.com/gin-gonic/gin"
)

type SystemController struct {
	BaseAPIController
}

// 直接插入排序
// 从第二个元素开始，取出元素，向前扫描，如果元素比当前元素小，则向前移动，直到找到合适的位置插入
// arr = [3, 5, 7, 2, 1, 9, 7]
// 第一个元素是 3，向前没有元素，保持不变
// 第二个元素是 5，向前扫描，元素 3 小于 5，保持不变
// 第三个元素是 7，向前扫描，元素 3 小于 7，元素 5 小于 7，保持不变
// 第四个元素是 2，向前扫描，元素 7 大于 2，向前移动
// 元素 5 大于 2，向前移动
// 元素 3 大于 2，向前移动
// 元素 2 插入到第一个位置
// arr = [2, 3, 5, 7, 1, 9, 7]
func insertSort(arr []int) []int {

	length := len(arr)
	for i := 1; i < length; i++ {
		temp := arr[i]
		j := i - 1

		for j >= 0 && arr[j] < temp {
			arr[j+1] = arr[j]
			j--
		}
		arr[j+1] = temp
	}

	return arr
}

// 直接选择排序
// 从第一个元素开始，遍历找到最小的值，然后与第一个元素交换
func selectSort(arr []int) []int {

	length := len(arr)
	for i := 0; i < length-1; i++ {
		min := arr[i]
		minIndex := i

		for j := i + 1; j < length; j++ {
			if arr[j] < min {
				// 记录下最小的值和对应的索引，寻找找到最小的值
				min = arr[j]
				minIndex = j
			}
		}

		if minIndex != i {
			arr[i], arr[minIndex] = arr[minIndex], arr[i]
		}
	}

	return arr
}

// 冒泡排序
func bubbleSort(arr []int) []int {
	length := len(arr)
	for i := 0; i < length-1; i++ {
		for j := 0; j < length-i-1; j++ {
			if arr[j] > arr[j+1] {
				arr[j], arr[j+1] = arr[j+1], arr[j]
			}
		}
	}
	return arr
}

// 希尔排序
func shellSort(arr []int) []int {
	length := len(arr)
	for gap := length / 2; gap > 0; gap /= 2 {
		for i := gap; i < length; i++ {
			temp := arr[i]
			j := i - gap
			for j >= 0 && arr[j] > temp {
				arr[j+gap] = arr[j]
				j -= gap
			}
			arr[j+gap] = temp
		}
	}
	return arr
}

// 堆排序
func heapSort(arr []int) []int {
	length := len(arr)
	// 从最后一个非叶子节点开始，向前遍历到根节点，构建大顶堆
	// 自下而上地进行 siftDown 操作
	for i := length/2 - 1; i >= 0; i-- {
		siftDown(arr, length, i)
	}
	// 排序
	// 自顶向下地进行 siftDown 操作
	for i := length - 1; i > 0; i-- {
		arr[0], arr[i] = arr[i], arr[0] // 交换堆顶元素和最后一个元素
		siftDown(arr, i, 0)             // 重新构建大顶堆
	}
	return arr
}

// siftDown 用于堆排序
//
//	arr = [16, 14, 10, 8, 7, 9, 3, 2, 4, 1]
func siftDown(arr []int, length int, i int) {
	largest := i     // 当前节点的位置
	left := 2*i + 1  // 左孩子的索引
	right := 2*i + 2 // 右孩子的索引
	if left < length && arr[left] > arr[largest] {
		largest = left
	}
	if right < length && arr[right] > arr[largest] {
		largest = right
	}
	if largest != i {
		arr[i], arr[largest] = arr[largest], arr[i]
		siftDown(arr, length, largest)
	}
}

// 二分查找/二分搜索
func binarySearch(arr []int, target int) int {
	low := 0
	high := len(arr) - 1
	for low <= high {
		mid := (low + high) / 2
		if arr[mid] == target {
			return mid
		} else if arr[mid] < target {
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	return -1
}

// 二叉搜索树
// 二叉搜索树（Binary Search Tree，BST）是一种特殊的二叉树，它满足以下性质：
// 1. 左子树中所有节点的值都小于根节点的值。
// 2. 右子树中所有节点的值都大于根节点的值。
// 3. 左子树和右子树也分别是二叉搜索树。
type Node struct {
	Value int
	Left  *Node
	Right *Node
}

// 生成新的二叉搜索树节点
func NewNode(value int) *Node {
	return &Node{Value: value}
}

// 插入节点
func InsertNode(root *Node, value int) *Node {
	if root == nil {
		return NewNode(value)
	}
	if value < root.Value {
		root.Left = InsertNode(root.Left, value)
	} else {
		root.Right = InsertNode(root.Right, value)
	}
	return root
}

// 二叉树搜索
func search(root *Node, value int) *Node {
	if root == nil {
		return nil
	}
	if value == root.Value {
		return root
	} else if value < root.Value {
		return search(root.Left, value)
	} else {
		return search(root.Right, value)
	}
}

// 二叉搜索树的生成
func buildBinarySearchTree(arr []int) *Node {
	var root *Node
	for _, value := range arr {
		root = InsertNode(root, value)
	}
	return root
}

// 前序遍历
func PreOrderTraversal(root *Node) {
	if root == nil {
		return
	}
	fmt.Printf("%d ", root.Value)
	PreOrderTraversal(root.Left)
	PreOrderTraversal(root.Right)
}

// 中序遍历
func InOrderTraversal(root *Node) {
	if root == nil {
		return
	}
	InOrderTraversal(root.Left)
	fmt.Printf("%d ", root.Value)
	InOrderTraversal(root.Right)
}

// 后序遍历
func PostOrderTraversal(root *Node) {
	if root == nil {
		return
	}
	PostOrderTraversal(root.Left)
	PostOrderTraversal(root.Right)
	fmt.Printf("%d ", root.Value)
}

// Health 用于容器联通性检查
func (ctrl *SystemController) Health(c *gin.Context) {

	// 直接插入排序
	arr := []int{3, 5, 7, 2, 1, 9, 7}
	sortedArr := insertSort(arr)
	fmt.Printf("%v\n", sortedArr)

	selectSortArr := selectSort(arr)
	fmt.Printf("%v\n", selectSortArr)

	// bubbleSortArr := bubbleSort(arr)
	// fmt.Printf("%v\n", bubbleSortArr)

	// shellSortArr := shellSort(arr)
	// fmt.Printf("%v\n", shellSortArr)

	dbStatus := "up"
	redisStatus := "up"
	rabbitStatus := "up"

	if database.SQLDB == nil || database.SQLDB.Ping() != nil {
		dbStatus = "down"
	}
	if pkgredis.Redis == nil || pkgredis.Redis.Ping() != nil {
		redisStatus = "down"
	}
	if !queue.Enabled() {
		rabbitStatus = "disabled"
	} else if queue.Ping() != nil {
		rabbitStatus = "down"
	}

	status := "ok"
	if dbStatus != "up" || redisStatus != "up" || (queue.Enabled() && rabbitStatus != "up") {
		status = "degraded"
	}

	response.JSON(c, gin.H{
		"status": status,
		"db":     dbStatus,
		"redis":  redisStatus,
		"rabbit": rabbitStatus,
		"time":   time.Now().Format(time.RFC3339),
	})
}

// PushTask 推送任务到 RabbitMQ
func (ctrl *SystemController) PushTask(c *gin.Context) {
	if !queue.Enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"message": "rabbitmq is disabled on this environment",
		})
		return
	}

	task := c.Query("task")
	if task == "" {
		task = fmt.Sprintf("test_task_%d", time.Now().Unix())
	}
	taskType := c.DefaultQuery("type", "video.generate")

	taskID, err := queue.PublishVideoTask(taskType, map[string]interface{}{
		"task": task,
	})
	if err != nil {
		response.Abort500(c, "push task failed: "+err.Error())
		return
	}

	response.JSON(c, gin.H{
		"message":   "task pushed",
		"task_id":   taskID,
		"task":      task,
		"task_type": taskType,
		"queue":     "rabbitmq",
	})
}

// DeepSeekTest backend 直接调用 DeepSeek，立即返回成功或失败。
func (ctrl *SystemController) DeepSeekTest(c *gin.Context) {
	prompt := c.Query("prompt")
	if prompt == "" {
		prompt = `Return strict JSON only: {"ok":true,"provider":"deepseek","msg":"ping"}`
	}
	testID := fmt.Sprintf("deepseek-test-%d", time.Now().UnixNano())

	content, err := deepseek.RunDeepSeekTest(deepseek.LoadConfig(), prompt)
	if err != nil {
		c.AbortWithStatusJSON(502, gin.H{
			"message":   "deepseek test failed",
			"test_id":   testID,
			"task_type": "deepseek.test",
			"error":     err.Error(),
		})
		return
	}

	response.JSON(c, gin.H{
		"message":   "deepseek test ok",
		"test_id":   testID,
		"task_type": "deepseek.test",
		"content":   content,
	})
}
