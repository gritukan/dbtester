#!/usr/bin/env bash
set -e

sudo apt install -y ansible

cat > /tmp/install-java.yml <<EOF
---
- name: a play that runs entirely on the ansible host
  hosts: localhost
  connection: local
  tasks:
  - name: Install add-apt-repostory
    become: yes
    apt: name=software-properties-common state=latest

  - name: Add Oracle Java Repository
    become: yes
    apt_repository: repo='ppa:webupd8team/java'

  - name: Accept Java 8 License
    become: yes
    debconf: name='oracle-java8-installer' question='shared/accepted-oracle-license-v1-1' value='true' vtype='select'

  - name: Install Oracle Java 8
    become: yes
    apt: name={{item}} state=latest
    with_items:
      - oracle-java8-installer
      - ca-certificates
      - oracle-java8-set-default

  - name: Print Java version
    command: java -version
    register: result
  - debug:
      var: result.stderr

  - name: Print JDK version
    command: javac -version
    register: result
  - debug:
      var: result.stderr
EOF
ansible-playbook /tmp/install-java.yml

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
curl -sf -o $HOME/zookeeper-tmp.zip https://www.apache.org/dyn/closer.lua/zookeeper/zookeeper-3.8.2/apache-zookeeper-3.8.2-bin.tar.gz
unzip $HOME/zookeeper-tmp.zip -d $HOME/zookeeper-tmp
mv $HOME/zookeeper-tmp/apache-zookeeper-3.8.2-beta $HOME/zookeeper
rm -f $HOME/zookeeper-tmp.zip
chmod -R 777 $HOME/zookeeper/
mkdir -p $HOME/zookeeper/zookeeper.data
touch $HOME/zookeeper/zookeeper.data/myid
chmod -R 777 $HOME/zookeeper/zookeeper.data/

cd $HOME/zookeeper
ls $HOME/zookeeper

echo "Done!"
