package main

import (
	"context"
	"fmt"
	"time"
	"workpool/pkg/types"

	"workpool" // 导入门面
)

func main() {
	// 1. 初始化池子
	// 配置：最大 5 个协程，队列长度 10，空闲 3 秒缩容
	p, err := workpool.NewPool(
		func(o *types.Options) {
			o.ExpiryTime = time.Second // 关键：缩短过期时间
			o.MaxWorkers = 5
		},
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("===== 测试 1: 正常高并发任务提交 =====")
	for i := 0; i < 20; i++ {
		id := i
		err := p.Submit(workpool.TaskFunc(func(ctx context.Context) error {
			fmt.Printf("[任务 %d] 正在运行, 当前活跃协程: %d\n", id, p.RunningWorkers())
			time.Sleep(500 * time.Millisecond) // 模拟耗时操作
			return nil
		}))

		if err != nil {
			fmt.Printf("[任务 %d] 提交失败: %v\n", id, err)
		}
	}
	fmt.Println("\n===== 测试 2: Panic 容灾测试 =====")
	p.Submit(workpool.TaskFunc(func(ctx context.Context) error {
		fmt.Println("[危险任务] 准备触发 Panic...")
		panic("Oops! 业务代码写炸了")
	}))

	// 3. 观察动态缩容
	fmt.Println("\n===== 测试 3: 等待任务完成并观察缩容 =====")
	fmt.Printf("等待前运行中的协程: %d\n", p.RunningWorkers())

	// 等待任务跑完
	time.Sleep(2 * time.Second)
	fmt.Printf("任务完成后，缩容前的协程: %d\n", p.RunningWorkers())

	// 等待超过 ExpiryTime (3秒)
	fmt.Println("等待 4 秒触发超时缩容...")
	time.Sleep(4 * time.Second)
	fmt.Printf("缩容后的协程数量: %d (预期应接近 0)\n", p.RunningWorkers())

	// 4. 优雅退出
	fmt.Println("\n===== 测试 4: 优雅退出 =====")
	p.Release()
	fmt.Println("工作池已安全关闭")
}
