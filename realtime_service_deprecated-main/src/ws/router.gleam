import dream/http/request
import dream/router
import ws/controller

pub fn create_router() {
  router.router()
  |> router.route(
    method: request.Get,
    path: "/ws",
    controller: controller.handle_upgrade,
    middleware: [],
  )
}
