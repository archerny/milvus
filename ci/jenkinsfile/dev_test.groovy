timeout(time: 20, unit: 'MINUTES') {
    try {
        dir ("${PROJECT_NAME}_test") {
            checkout([$class: 'GitSCM', branches: [[name: "${SEMVER}"]], doGenerateSubmoduleConfigurations: false, extensions: [], submoduleCfg: [], userRemoteConfigs: [[credentialsId: "${params.GIT_USER}", url: "git@192.168.1.105:Test/milvus_test.git", name: 'origin', refspec: "+refs/heads/${SEMVER}:refs/remotes/origin/${SEMVER}"]]])
            sh 'python3 -m pip install -r requirements.txt'
            sh "pytest . --alluredir=test_out --ip ${env.JOB_NAME}-${env.BUILD_NUMBER}-milvus-gpu-engine.kube-opt.svc.cluster.local"
        }

        // mysql database backend test
        load "${env.WORKSPACE}/ci/jenkinsfile/cleanup_dev.groovy"
        if (!fileExists('milvus-helm')) {
            dir ("milvus-helm") {
                checkout([$class: 'GitSCM', branches: [[name: "${SEMVER}"]], doGenerateSubmoduleConfigurations: false, extensions: [], submoduleCfg: [], userRemoteConfigs: [[credentialsId: "${params.GIT_USER}", url: "git@192.168.1.105:megasearch/milvus-helm.git", name: 'origin', refspec: "+refs/heads/${SEMVER}:refs/remotes/origin/${SEMVER}"]]])
            }
        }
        dir ("milvus-helm") {
            dir ("milvus/milvus-gpu") {
                sh "helm install --set engine.image.tag=${DOCKER_VERSION} --set expose.type=clusterIP --name ${env.JOB_NAME}-${env.BUILD_NUMBER} -f ci/db_backend/mysql_values.yaml --version 0.3.1 ."
            }
        }
        dir ("${PROJECT_NAME}_test") {
            sh "pytest . --alluredir=test_out --ip ${env.JOB_NAME}-${env.BUILD_NUMBER}-milvus-gpu-engine.kube-opt.svc.cluster.local"
        }
    } catch (exc) {
        echo 'Milvus Test Failed !'
        throw exc
    }
}
