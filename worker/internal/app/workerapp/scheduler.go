package workerapp

import (
	"worker/internal/app/task"
	idiompipeline "worker/internal/app/workflow/idiom"
	podcastaudiopipeline "worker/internal/app/workflow/podcast/audio"
	podcastcomposepipeline "worker/internal/app/workflow/podcast/compose"
	podcastpagepipeline "worker/internal/app/workflow/podcast/page"
	practicalaudiopipeline "worker/internal/app/workflow/practical/audio"
	practicalcomposepipeline "worker/internal/app/workflow/practical/compose"
	practicalimagepipeline "worker/internal/app/workflow/practical/image"
	practicalpagepipeline "worker/internal/app/workflow/practical/page"
	uploadpipeline "worker/internal/app/workflow/upload"
)

func SchedulerForRole(role string) map[string]task.TaskHandler {
	scheduler := make(map[string]task.TaskHandler)
	normalizedRole := task.NormalizeWorkerRole(role)

	if normalizedRole == task.WorkerRoleAll || normalizedRole == task.WorkerRoleMain {
		scheduler["plan.v1"] = idiompipeline.HandlePlan
		scheduler["scene.generate.v1"] = idiompipeline.HandleSceneGenerate
		scheduler["compose.v1"] = idiompipeline.HandleProjectCompose
		scheduler["practical.audio.generate.v1"] = practicalaudiopipeline.HandleGenerate
		scheduler["practical.image.generate.v1"] = practicalimagepipeline.HandleGenerate
		scheduler["practical.compose.render.v1"] = practicalcomposepipeline.HandleComposeRender
		scheduler["practical.page.persist.v1"] = practicalpagepipeline.HandlePersist
		scheduler["podcast.audio.generate.v1"] = podcastaudiopipeline.HandleGenerate
		scheduler["podcast.compose.render.v1"] = podcastcomposepipeline.HandleComposeRender
		scheduler["podcast.compose.finalize.v1"] = podcastcomposepipeline.HandleComposeFinalize
		scheduler["upload.v1"] = uploadpipeline.HandleUploadTask
		scheduler["podcast.page.persist.v1"] = podcastpagepipeline.HandlePersist
	}

	if normalizedRole == task.WorkerRoleAll || normalizedRole == task.WorkerRoleAlign {
		scheduler["practical.audio.align.v1"] = practicalaudiopipeline.HandleAlign
		scheduler["podcast.audio.align.v1"] = podcastaudiopipeline.HandleAlign
	}

	return scheduler
}
