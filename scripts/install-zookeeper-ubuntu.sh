#!/usr/bin/env bash
set -e

sudo apt install default-jdk
sudo apt install default-jre
sudo apt install unzip

java -version
javac -version

<<COMMENT
ZK_VERSION=3.4.9
rm -rf $HOME/zookeeper
curl -sf -o /tmp/zookeeper-$ZK_VERSION.tar.gz -L https://www.apache.o-zookeeper-consulrg/dist/zookeeper/zookeeper-$ZK_VERSION/zookeeper-$ZK_VERSION.tar.gz
tar -xzf /tmp/zookeeper-$ZK_VERSION.tar.gz -C /tmp/
mv /tmp/zookeeper-$ZK_VERSION /tmp/zookeeper
mv /tmp/zookeeper $HOME/
chmod -R 777 $HOME/zookeeper/
mkdir -p $HOME/zookeeper/zookeeper.data
touch $HOME/zookeeper/zookeeper.data/myid
chmod -R 777 $HOME/zookeeper/zookeeper.data/
COMMENT

# official zookeeper-3.5.3-beta.tar.gz is corrupted
# errors when tar out, just use temporary zipped files
rm -rf $HOME/zookeeper
rm -rf $HOME/zookeeper-tmp
rm -f $HOME/zookeeper-tmp.zip
curl -L -sf -o $HOME/zookeeper-tmp.tar.gz https://dlcdn.apache.org/zookeeper/zookeeper-3.8.2/apache-zookeeper-3.8.2-bin.tar.gz
mkdir $HOME/zookeeper-tmp
tar xvzf $HOME/zookeeper-tmp.tar.gz -C $HOME/zookeeper-tmp
mv $HOME/zookeeper-tmp/apache-zookeeper-3.8.2-bin $HOME/zookeeper
rm -f $HOME/zookeeper-tmp.tar.gz
chmod -R 777 $HOME/zookeeper/
mkdir -p $HOME/zookeeper/zookeeper.data
touch $HOME/zookeeper/zookeeper.data/myid
chmod -R 777 $HOME/zookeeper/zookeeper.data/

cd $HOME/zookeeper
ls $HOME/zookeeper

echo "Done!"

