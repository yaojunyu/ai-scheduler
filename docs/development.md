# AI-Scheduler 开发流程

以下默认 `PROJECT` 为本项目开发的根目录

## 构建

1. 构建linux环境下的二进制包
    - 交叉编译方式构建:  
        此方法需要的 go 版本最好在1.12.1, 需要支持gcc的环境，需要下载依赖  
       `$ cd $PROJECT && make rel_bins`

    - docker 方式构建：
        此方法需要在有依赖变动时更新 build 镜像
       `$ cd $PROJECT && make docker_build`

    构建得到的二进制包会被放到 `./_output` 目录下

2. 构建运行镜像

    `$ cd $PROJECT && make images`

3. 推送

    `$ cd $PROJECT && make push`

4. 清理

    `$ cd $PROJECT && make clean`

## 开发

TODO：待完成