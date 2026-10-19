# -*- coding: utf-8 -*-
from mod.common.mod import Mod
import mod.server.extraServerApi as serverApi

@Mod.Binding(name="YouyuanObserver", version="0.10.29")
class YouyuanObserver(object):
    @Mod.InitServer()
    def server_init(self):
        serverApi.RegisterSystem("YouyuanBridge", "ObserverV1", "youyuan_observer.server.ObserverServer")
