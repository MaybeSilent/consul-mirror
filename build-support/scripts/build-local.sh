#!/bin/bash
# basename取输入文件路径最后的文件目录或文件名
# dirname取输入文件路径的上一层文件夹
# BASH_SOURCE 相当于source路径
SCRIPT_NAME="$(basename ${BASH_SOURCE[0]})"
pushd $(dirname ${BASH_SOURCE[0]}) > /dev/null
SCRIPT_DIR=$(pwd)
pushd ../.. > /dev/null
SOURCE_DIR=$(pwd)
popd > /dev/null
pushd ../functions > /dev/null
FN_DIR=$(pwd) ## 对应function dir
popd > /dev/null
popd > /dev/null

source "${SCRIPT_DIR}/functions.sh"
# >>>>>>>>>>>> 为各个变量进行赋值操作，通过source引入functions.sh中的函数并执行相关命令
function usage {
cat <<-EOF
Usage: ${SCRIPT_NAME} [<options ...>]

Description:
   This script will build the Consul binary on the local system.
   All the requisite tooling must be installed for this to be
   successful.

Options:
                       
   -s | --source     DIR         Path to source to build.
                                 Defaults to "${SOURCE_DIR}"
                                 
   -o | --os         OSES        Space separated string of OS
                                 platforms to build.
                                 
   -a | --arch       ARCH        Space separated string of
                                 architectures to build.
   
   -h | --help                   Print this help text.
EOF
}

function err_usage {
   err "$1"
   err ""
   err "$(usage)"
}

function main {
   declare sdir="${SOURCE_DIR}"
   declare build_os=""
   declare build_arch=""
   
   
   while test $# -gt 0
   do
      case "$1" in # shell的case，)右圆括号表示开始，;;双引号表示结束
         -h | --help )
            usage # usage函数调用
            return 0
            ;;
         -s | --source )
            if test -z "$2"
            then
               err_usage "ERROR: option -s/--source requires an argument"
               return 1
            fi
            
            if ! test -d "$2" # 文件存在且目录为真
            then
               err_usage "ERROR: '$2' is not a directory and not suitable for the value of -s/--source"
               return 1
            fi
            
            sdir="$2"
            shift 2 # shell参数进行移动
            ;;
         -o | --os )
            if test -z "$2"
            then
               err_usage "ERROR: option -o/--os requires an argument"
               return 1
            fi
            
            build_os="$2"
            shift 2
            ;;
         -a | --arch )
            if test -z "$2"
            then
               err_usage "ERROR: option -a/--arch requires an argument"
               return 1
            fi
            
            build_arch="$2"
            shift 2
            ;;
         * ) # 未知参数
            err_usage "ERROR: Unknown argument: '$1'"
            return 1
            ;;
      esac
   done

   ## 调用编译函数
   build_consul_local "${sdir}" "${build_os}" "${build_arch}" || return 1
   
   return 0
}

## $@与$*的区别，都表示传递给函数或脚本的所有参数
main "$@"
## $?获取上一个命令的退出状态
exit $?