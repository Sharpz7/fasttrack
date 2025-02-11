package helpers

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/suite"

	"github.com/G-Research/fasttrackml/pkg/api/mlflow/common"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/config"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/dao/models"
	"github.com/G-Research/fasttrackml/pkg/database"
	"github.com/G-Research/fasttrackml/pkg/server"
	"github.com/G-Research/fasttrackml/tests/integration/golang/fixtures"
)

type BaseTestSuite struct {
	suite.Suite
	server                      server.Server
	db                          database.DBProvider
	setupHooks                  []func()
	tearDownHooks               []func()
	AIMClient                   func() *HttpClient
	MlflowClient                func() *HttpClient
	AdminClient                 func() *HttpClient
	AppFixtures                 *fixtures.AppFixtures
	RunFixtures                 *fixtures.RunFixtures
	TagFixtures                 *fixtures.TagFixtures
	MetricFixtures              *fixtures.MetricFixtures
	ContextFixtures             *fixtures.ContextFixtures
	ParamFixtures               *fixtures.ParamFixtures
	ProjectFixtures             *fixtures.ProjectFixtures
	DashboardFixtures           *fixtures.DashboardFixtures
	ExperimentFixtures          *fixtures.ExperimentFixtures
	DefaultExperiment           *models.Experiment
	NamespaceFixtures           *fixtures.NamespaceFixtures
	DefaultNamespace            *models.Namespace
	ResetOnSubTest              bool
	SkipCreateDefaultNamespace  bool
	SkipCreateDefaultExperiment bool
}

func (s *BaseTestSuite) runSetupHooks() {
	for _, hook := range s.setupHooks {
		hook()
	}
}

func (s *BaseTestSuite) runTearDownHooks() {
	for _, hook := range s.tearDownHooks {
		hook()
	}
}

func (s *BaseTestSuite) initLogger() {
	levelString := GetLogLevel()
	level, err := logrus.ParseLevel(levelString)
	s.Require().Nil(err)
	logrus.SetLevel(level)
}

func (s *BaseTestSuite) initDB() {
	dsn, err := GenerateDatabaseURI(s.T(), GetDatabaseBackend())
	s.Require().Nil(err)

	s.db, err = database.NewDBProvider(
		dsn,
		1*time.Second,
		20,
	)
	s.Require().Nil(err)
}

func (s *BaseTestSuite) initFixtures() {
	db := s.db.GormDB()

	appFixtures, err := fixtures.NewAppFixtures(db)
	s.Require().Nil(err)
	s.AppFixtures = appFixtures

	dashboardFixtures, err := fixtures.NewDashboardFixtures(db)
	s.Require().Nil(err)
	s.DashboardFixtures = dashboardFixtures

	experimentFixtures, err := fixtures.NewExperimentFixtures(db)
	s.Require().Nil(err)
	s.ExperimentFixtures = experimentFixtures

	metricFixtures, err := fixtures.NewMetricFixtures(db)
	s.Require().Nil(err)
	s.MetricFixtures = metricFixtures

	contextFixtures, err := fixtures.NewContextFixtures(db)
	s.Require().Nil(err)
	s.ContextFixtures = contextFixtures

	namespaceFixtures, err := fixtures.NewNamespaceFixtures(db)
	s.Require().Nil(err)
	s.NamespaceFixtures = namespaceFixtures

	projectFixtures, err := fixtures.NewProjectFixtures(db)
	s.Require().Nil(err)
	s.ProjectFixtures = projectFixtures

	paramFixtures, err := fixtures.NewParamFixtures(db)
	s.Require().Nil(err)
	s.ParamFixtures = paramFixtures

	runFixtures, err := fixtures.NewRunFixtures(db)
	s.Require().Nil(err)
	s.RunFixtures = runFixtures

	tagFixtures, err := fixtures.NewTagFixtures(db)
	s.Require().Nil(err)
	s.TagFixtures = tagFixtures
}

func (s *BaseTestSuite) closeDB() {
	s.Require().Nil(s.db.Close())
}

func (s *BaseTestSuite) startServer() {
	var err error
	s.server, err = server.NewServer(context.Background(), &config.ServiceConfig{
		DatabaseURI:           s.db.Dsn(),
		DatabasePoolMax:       10,
		DatabaseSlowThreshold: 1 * time.Second,
		DatabaseMigrate:       true,
		DefaultArtifactRoot:   s.T().TempDir(),
		S3EndpointURI:         GetS3EndpointUri(),
		GSEndpointURI:         GetGSEndpointUri(),
	})
	s.Require().Nil(err)

	s.AIMClient = func() *HttpClient {
		return NewAimApiClient(s.server)
	}
	s.MlflowClient = func() *HttpClient {
		return NewMlflowApiClient(s.server)
	}
	s.AdminClient = func() *HttpClient {
		return NewAdminApiClient(s.server)
	}
}

func (s *BaseTestSuite) stopServer() {
	s.Require().Nil(s.server.ShutdownWithTimeout(5 * time.Second))
}

func (s *BaseTestSuite) setupDatabase() {
	s.resetDatabase()

	if !s.SkipCreateDefaultNamespace {
		var err error
		s.DefaultNamespace, err = s.NamespaceFixtures.CreateNamespace(context.Background(), &models.Namespace{
			ID:                  1,
			Code:                "default",
			DefaultExperimentID: common.GetPointer(int32(0)),
		})
		s.Require().Nil(err)

		if !s.SkipCreateDefaultExperiment {
			s.DefaultExperiment, err = s.ExperimentFixtures.CreateExperiment(context.Background(), &models.Experiment{
				ID:             common.GetPointer[int32](0),
				Name:           "Default",
				LifecycleStage: models.LifecycleStageActive,
				NamespaceID:    s.DefaultNamespace.ID,
			})
			s.Require().Nil(err)

			s.DefaultNamespace.DefaultExperimentID = s.DefaultExperiment.ID
			_, err = s.NamespaceFixtures.UpdateNamespace(context.Background(), s.DefaultNamespace)
			s.Require().Nil(err)
		}
	}
}

func (s *BaseTestSuite) resetDatabase() {
	s.Require().Nil(s.NamespaceFixtures.TruncateTables())
}

func (s *BaseTestSuite) AddSetupHook(hook func()) {
	s.setupHooks = append(s.setupHooks, hook)
}

func (s *BaseTestSuite) AddTearDownHook(hook func()) {
	s.tearDownHooks = append([]func(){hook}, s.tearDownHooks...)
}

func (s *BaseTestSuite) SetupSuite() {
	s.initLogger()
	s.initDB()
	s.initFixtures()
	s.AddSetupHook(s.startServer)
	s.AddSetupHook(s.setupDatabase)
	s.AddTearDownHook(s.resetDatabase)
	s.AddTearDownHook(s.stopServer)
}

func (s *BaseTestSuite) TearDownSuite() {
	s.closeDB()
}

func (s *BaseTestSuite) SetupTest() {
	if !s.ResetOnSubTest {
		s.runSetupHooks()
	}
}

func (s *BaseTestSuite) SetupSubTest() {
	if s.ResetOnSubTest {
		s.runSetupHooks()
	}
}

func (s *BaseTestSuite) TearDownTest() {
	if !s.ResetOnSubTest {
		s.runTearDownHooks()
	}
}

func (s *BaseTestSuite) TearDownSubTest() {
	if s.ResetOnSubTest {
		s.runTearDownHooks()
	}
}
