# Executor Domain 

## Description

提供一层抽象，负责对流水线执行快照进行分发、调度、执行。

## Dispatcher

对任务进行分发，将pipeline的执行快照分发给各个执行器，
执行器通过注册模式被分发器发现，实现分布式架构。

    主要能力:

    - 任务分发
    - 异常恢复
        - 未完成任务节点重新分发
        - 悬挂(孤儿)节点处理



## Scheduler

接收到任务后，对任务进行调度处理，分发给实际的工作引擎，
与工作引擎进行直接交互

    - 主要能力
    
    - 建立与分发器的watch长连接,类似k8s的控制器逻辑

    

