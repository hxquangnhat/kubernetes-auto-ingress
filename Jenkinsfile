pipeline {
  agent any
  stages {
    stage('get k8s-client-go') {
      steps {
        parallel(
          "get k8s-client-go": {
            sh 'go get k8s.io/client-go/...'
            
          },
          "get logsirup": {
            sh 'go get github.com/Sirupsen/logrus'
            
          }
        )
      }
    }
    stage('make') {
      steps {
        sh 'make'
      }
    }
  }
}