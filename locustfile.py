from locust import HttpLocust, TaskSet


def get_price(l):
    l.client.get("/")


class UserBehavior(TaskSet):
    tasks = {get_price: 1}

    def on_start(self):
        pass

    def on_stop(self):
        pass


class WebsiteUser(HttpLocust):
    task_set = UserBehavior
    # min_wait = 5000
    # max_wait = 9000