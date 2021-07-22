# Testing guide

The purpose of this document is to provide start guide to set up, run and extend e2e test suite.

### Prerequisites/Dependencies

- Install Ginkgo and Gomega 

      go get github.com/onsi/ginkgo/ginkgo
      go get github.com/onsi/gomega/...
    <em>Note:</em> This might already be installed.
   
- Define env variables
    - Required environment variables
      
          export AWSBUCKETNAME=<aws_bucket_name>
          export AWSACCESSKEY=<aws_access_key>
          export AWSSECRETKEY=<aws_secret_key>
          export AWSREGION=<aws_bucket_region>
          export EXPOSEDREGISTRYPATH=<exposed_registry_path>
          export SOURCEURL=<source_cluster_url>
          export BACKUPSTORAGEPROVIDER=<backup_storage_provider_name>
          export SOURCECONFIG=<path_to_kubeconfig_of_source_cluster>
          export HOSTCONFIG=<path_to_kubeconfig_of_host_cluster>
    
    - Optional environment variables
        
          export VELERO_PLUGIN_FQIN=<openshift_velero_plugin_image> # this is a string, default value is "quay.io/konveyor/openshift-velero-plugin:latest"
          export MIG_CONTROLLER_IMAGE_FQIN=<mig_controller_image>   # this is a string, default value is "quay.io/konveyor/mig-controller:latest"
          export RSYNC_TRANSFER_IMAGE_FQIN=<rsync_transfer_image>   # this is a string, default value is "quay.io/konveyor/rsync-transfer:latest"
          export MIG_POD_LIMIT=<migration_pod_limit>                # this is an integer, default value is 100
          export CLUSTER_NAME=<host_cluster_name>                   # this is a string, default value is "host"
          export RESTIC_TIMEOUT=<restic_timeout>                    # this is a string, default value is "1h"
          export MIGRATION_VELERO=<flag_to_use_velero>              # this is a bool, default value is true
          export MIG_NAMESPACE_LIMIT=<limit_namespace_migration>    # this is an integer, default value is 10
          export MIG_PV_LIMIT=<mig_pv_limit>                        # this is an integer, default value is 100
          export VERSION=<version>                                  # this is a string, defaut value is "latest"


### Command to run the test suite

```
# this command should be run from root of the mig-controller repo
make e2e-test 
```

### Structure of the current suite

#### Set-up for the test suite
`BeforeSuite` of `test_suite_test.go` is responsible for setting up the environment to the point where all the dependencies to create a plan and run a migration. This includes installing controller if not present, creation of `migStorage` and `migCluster` and making sure these are in the desired state. Currently `migSorage` is limited to `aws s3` bucket, any other type of `migStorage` creation will be added in future.   
#### Tests pertaining to features/scenarios
`Describe` in `tests_test.go` should pertain to one feature/scenario. As of now, one test case to test BZ-1965421 is included in the e2e suite.

Each feature might need some set up of their own, such as a `migPlan`, `migMigration`.  These dependencies can be fulfilled with helpers `BeforeEach`, `JustBeforeEach` and clean up of these dependencies should be handled in `AfterEach`, `JustAfterEach`. `It` is the lowest level block where the assertion of the desired properties or behavior should be made (<em><b>Note: </b>`BeforeEach/AfterEach` gets invoked for every `It`</em>). Detailed explanation on how these blocks are used can be found [here](https://onsi.github.io/ginkgo/#structuring-your-specs). Explanation on how to assert for different errors/values can be found [here](https://onsi.github.io/gomega/#making-assertions) and [here](https://onsi.github.io/gomega/#making-assertions-in-helper-functions).

<em><b>Note:</b></em> While adding new tests, making sure to not create objects that might be in conflict with other tests and cleaning up after the test is executed is of utmost importance.

#### Clean-up for the test suite
`AfterSuite` of `test_suite_test.go` is responsible for cleaning up the environment. It deletes `migCluster`, `migStorage` and deletes the installer if it was created by the suite.
 
### How to extend the test-suite

Each test case for a feature/scenario should typically be of the following structure.
```
Describe()
    BeforeEach()
    AfterEach()
    JustBeforeEach()
    Context()
        It()
    
    Context()
        BeforeEach() // specific to this context
        It()        
```

To keep the code readable, it is advisable to define the objects needed for the tests within a function in `test_helper.go` and use them while creating within the test.

### Limitations
- Currently `aws s3` is configured for `migStorage`.

<em>This document is subject to change.</em>