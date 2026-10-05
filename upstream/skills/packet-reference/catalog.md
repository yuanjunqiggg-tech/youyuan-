# 当前内核协议包目录

215种实际注册协议包。常用字段附中文说明，原始内核注释保留，网易扩展需真实样本核对。

## 1 · Login · 登录

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：Login is sent when the client initially tries to join the server. It is the first packet sent and contains information specific to the player.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `client_protocol` | `int32` | ClientProtocol is the protocol version of the player. The player is disconnected if the protocol is incompatible with the protocol of the server. It has been superseded by the protocol version sent in the RequestNetworkSettings packet, so this should no longer be used by the server. |
| `connection_request` | `[]uint8` | ConnectionRequest is a string containing information about the player and JWTs that may be used to verify if the player is connected to XBOX Live. The connection request also contains the necessary client public key to initiate encryption. |

## 2 · PlayStatus · 登录与生成状态

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：登录是否成功、玩家是否生成；状态码由当前协议定义。
- 本内核原始说明：PlayStatus is sent by the server to update a player on the play status. This includes failed statuses due to a mismatched version, but also success statuses.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `status` | `int32` | Status is the status of the packet. It is one of the constants found above. |

## 3 · ServerToClientHandshake · 服务器握手

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：连接内核的会话加密协商，不由插件重复发送。
- 本内核原始说明：ServerToClientHandshake is sent by the server to the client to complete the key exchange in order to initialise encryption on client and server side. It is followed up by a ClientToServerHandshake packet from the client.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `jwt` | `[]uint8` | JWT is a raw JWT token containing data such as the public key from the server, the algorithm used and the server's token. It is used for the client to produce a shared secret. |

## 4 · ClientToServerHandshake · 客户端握手

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：客户端确认会话协商，交由连接内核处理。
- 本内核原始说明：ClientToServerHandshake is sent by the client in response to a ServerToClientHandshake packet sent by the server. It is the first encrypted packet in the login handshake and serves as a confirmation that encryption is correctly initialised client side.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|

## 5 · Disconnect · 断开

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：Disconnect may be sent by the server to disconnect the client using an optional message to send as the disconnect screen.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `reason` | `int32` | Reason is the reason for the disconnection. It seems as if this field has no use other than for telemetry reasons as it does not affect the message that gets displayed on the disconnect screen. |
| `hide_disconnection_screen` | `bool` | HideDisconnectionScreen specifies if the disconnection screen should be hidden when the client is disconnected, meaning it will be sent directly to the main menu. |
| `message` | `string` | Message is an optional message to show when disconnected. This message is only written if the HideDisconnectionScreen field is set to true. |
| `filtered_message` | `string` | FilteredMessage is a filtered version of Message with all the profanity removed. The client will use this over Message if this field is not empty and they have the "Filter Profanity" setting enabled. |

## 6 · ResourcePacksInfo · 资源包信息

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：服务器提供资源包列表；可检查服务器要求加载的资源包。
- 本内核原始说明：ResourcePacksInfo is sent by the server to inform the client on what resource packs the server has. It sends a list of the resource packs it has and basic information on them like the version and description.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `texture_pack_required` | `bool` | TexturePackRequired specifies if the client must accept the texture packs the server has in order to join the server. If set to true, the client gets the option to either download the resource packs and join, or quit entirely. Behaviour packs never have to be downloaded. |
| `has_addons` | `bool` | HasAddons specifies if any of the resource packs contain addons in them. If set to true, only clients that support addons will be able to download them. |
| `has_scripts` | `bool` | HasScripts specifies if any of the resource packs contain scripts in them. If set to true, only clients that support scripts will be able to download them. |
| `force_disable_vibrant_visuals` | `bool` | ForceDisableVibrantVisuals specifies if the vibrant visuals feature should be forcibly disabled on the server. If set to true, the server will ensure that vibrant visuals are not enabled, regardless of the client's settings. |
| `world_template_uuid` | `uuid.UUID` | WorldTemplateUUID is the UUID of the template that has been used to generate the world. Templates can be downloaded from the marketplace or installed via '.mctemplate' files. If the world was not generated from a template, this field is empty. |
| `world_template_version` | `string` | WorldTemplateVersion is the version of the world template that has been used to generate the world. If the world was not generated from a template, this field is empty. |
| `texture_packs` | `[]protocol.TexturePackInfo` | TexturePacks is a list of texture packs that the client needs to download before joining the server. The order of these texture packs is not relevant in this packet. It is however important in the ResourcePackStack packet. |

### texture_packs 嵌套字段

- `uuid` (`uuid.UUID`)：UUID is the UUID of the texture pack. Each texture pack downloaded must have a different UUID in order for the client to be able to handle them properly.
- `version` (`string`)：Version is the version of the texture pack. The client will cache texture packs sent by the server as long as they carry the same version. Sending a texture pack with a different version than previously will force the client to re-download it.
- `size` (`uint64`)：Size is the total size in bytes that the texture pack occupies. This is the size of the compressed archive (zip) of the texture pack.
- `content_key` (`string`)：ContentKey is the key used to decrypt the behaviour pack if it is encrypted. This is generally the case for marketplace texture packs.
- `sub_pack_name` (`string`)：SubPackName ...
- `content_identity` (`string`)：ContentIdentity is another UUID for the resource pack, and is generally set for marketplace texture packs. It is also required for client-side validations when the resource pack is encrypted.
- `has_scripts` (`bool`)：HasScripts specifies if the texture packs has any scripts in it. A client will only download the behaviour pack if it supports scripts, which, up to 1.11, only includes Windows 10.
- `addon_pack` (`bool`)：AddonPack specifies if the texture pack is from an addon.
- `rtxenabled` (`bool`)：RTXEnabled specifies if the texture pack uses the raytracing technology introduced in 1.16.200.
- `download_url` (`string`)：DownloadURL is a URL that the client can use to download the pack instead of the server sending it in chunks, which it will continue to do if this field is left empty.

## 7 · ResourcePackStack · 资源包栈

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ResourcePackStack is sent by the server to send the order in which resource packs and behaviour packs should be applied (and downloaded) by the client.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `texture_pack_required` | `bool` | TexturePackRequired specifies if the client must accept the texture packs the server has in order to join the server. If set to true, the client gets the option to either download the resource packs and join, or quit entirely. Behaviour packs never have to be downloaded. |
| `behaviour_packs` | `[]protocol.StackResourcePack` | BehaviourPack is a list of behaviour packs that the client needs to download before joining the server. All of these behaviour packs will be applied together, and the order does not necessarily matter. |
| `texture_packs` | `[]protocol.StackResourcePack` | TexturePacks is a list of texture packs that the client needs to download before joining the server. The order of these texture packs specifies the order that they are applied in on the client side. The first in the list will be applied first. |
| `base_game_version` | `string` | BaseGameVersion is the vanilla version that the client should set its resource pack stack to. |
| `experiments` | `[]protocol.ExperimentData` | Experiments holds a list of experiments that are either enabled or disabled in the world that the player spawns in. It is not clear why experiments are sent both here and in the StartGame packet. |
| `experiments_previously_toggled` | `bool` | ExperimentsPreviouslyToggled specifies if any experiments were previously toggled in this world. It is probably used for some kind of metrics. |
| `include_editor_packs` | `bool` | IncludeEditorPacks specifies if vanilla editor packs should be included in the resource pack stack when connecting to an editor world. |

### behaviour_packs 嵌套字段

- `uuid` (`string`)：UUID is the UUID of the resource pack. Each resource pack downloaded must have a different UUID in order for the client to be able to handle them properly.
- `version` (`string`)：Version is the version of the resource pack. The client will cache resource packs sent by the server as long as they carry the same version. Sending a resource pack with a different version than previously will force the client to re-download it.
- `sub_pack_name` (`string`)：SubPackName ...

### texture_packs 嵌套字段

- `uuid` (`string`)：UUID is the UUID of the resource pack. Each resource pack downloaded must have a different UUID in order for the client to be able to handle them properly.
- `version` (`string`)：Version is the version of the resource pack. The client will cache resource packs sent by the server as long as they carry the same version. Sending a resource pack with a different version than previously will force the client to re-download it.
- `sub_pack_name` (`string`)：SubPackName ...

### experiments 嵌套字段

- `name` (`string`)：Name is the name of the experiment.
- `enabled` (`bool`)：Enabled specifies if the experiment is enabled. Vanilla typically always sets this to true for any experiments sent.

## 8 · ResourcePackClientResponse · 资源包客户端响应

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ResourcePackClientResponse is sent by the client in response to resource packets sent by the server. It is used to let the server know what action needs to be taken for the client to have all resource packs ready and set.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `response` | `uint8` | Response is the response type of the response. It is one of the constants found above. |
| `packs_to_download` | `[]string` | PacksToDownload is a list of resource pack UUIDs combined with their version that need to be downloaded (for example SomePack_1.0.0), if the Response field is PackResponseSendPacks. |

## 9 · Text · 聊天与翻译消息

- 分组：消息与显示
- 方向：双向，具体TextType有别
- 用法与边界：SourceName/Message/TextType/Parameters；聊天、系统翻译与命令反馈不同，死亡翻译需检查真实key。
- 本内核原始说明：Text is sent by the client to the server to send chat messages, and by the server to the client to forward or send messages, which may be chat, popups, tips etc.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `text_type` | `uint8` | TextType is the type of the text sent. When a client sends this to the server, it should always be TextTypeChat. If the server sends it, it may be one of the other text types above. |
| `needs_translation` | `bool` | NeedsTranslation specifies if any of the messages need to be translated. It seems that where % is found in translatable text types, these are translated regardless of this bool. Translatable text types include TextTypeTranslation, TextTypeTip, TextTypePopup and TextTypeJukeboxPopup. |
| `source_name` | `string` | SourceName is the name of the source of the messages. This source is displayed in text types such as the TextTypeChat and TextTypeWhisper, where typically the username is shown. |
| `message` | `string` | Message is the message of the packet. This field is set for each TextType and is the main component of the packet. |
| `parameters` | `[]string` | Parameters is a list of parameters that should be filled into the message. These parameters are only written if the type of the packet is TextTypeTranslation, TextTypeTip, TextTypePopup or TextTypeJukeboxPopup. |
| `xuid` | `string` | XUID is the XBOX Live user ID of the player that sent the message. It is only set for packets of TextTypeChat. When sent to a player, the player will only be shown the chat message if a player with this XUID is present in the player list and not muted, or if the XUID is empty. |
| `platform_chat_id` | `string` | PlatformChatID is an identifier only set for particular platforms when chatting (presumably only for Nintendo Switch). It is otherwise an empty string, and is used to decide which players are able to chat with each other. |
| `filtered_message` | `string` | FilteredMessage is a filtered version of Message with all the profanity removed. The client will use this over Message if this field is not empty and they have the "Filter Profanity" setting enabled. |
| `unknown1` | `[]string` | PhoenixBuilder specific fields. Author: Liliya233, Happy2018new  The following fields are NetEase specific. |
| `unknown2` | `string` |  |

## 10 · SetTime · 世界时间同步

- 分组：世界与结构
- 方向：服务器→客户端
- 用法与边界：仅按服务器真实Time观测游戏时间和TPS；玩家手动/time会改时间，不用现实时间硬算永久昼夜。
- 本内核原始说明：SetTime is sent by the server to update the current time client-side. The client actually advances time client-side by itself, so this packet does not need to be sent each tick. It is merely a means of synchronising time between server and client.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `time` | `int32` | Time is the current time. The time is not limited to 24000 (time of day), but continues progressing after that. |

## 11 · StartGame · 开始游戏

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：StartGame is sent by the server to send information about the world the player will be spawned in. It contains information about the position the player spawns in, and information about the world in general such as its game rules.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_unique_id` | `int64` | 实体唯一ID，与运行ID不同。 EntityUniqueID is the unique ID of the player. The unique ID is a value that remains consistent across different sessions of the same world, but most servers simply fill the runtime ID of the entity out for this field. |
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the player. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `player_game_mode` | `int32` | PlayerGameMode is the game mode the player currently has. It is a value from 0-4, with 0 being survival mode, 1 being creative mode, 2 being adventure mode, 3 being survival spectator and 4 being creative spectator. This field may be set to 5 to make the client fall back to the game mode set in the WorldGameMode field. |
| `player_position` | `mgl32.Vec3` | PlayerPosition is the spawn position of the player in the world. In servers this is often the same as the world's spawn position found below. |
| `pitch` | `float32` | 俯仰角。 Pitch is the vertical rotation of the player. Facing straight forward yields a pitch of 0. Pitch is measured in degrees. |
| `yaw` | `float32` | 偏航角。 Yaw is the horizontal rotation of the player. Yaw is also measured in degrees. |
| `world_seed` | `int64` | WorldSeed is the seed used to generate the world. Unlike in PC edition, the seed is a 32bit integer here. |
| `spawn_biome_type` | `int16` | SpawnBiomeType specifies if the biome that the player spawns in is user defined (through behaviour packs) or builtin. See the constants above. |
| `user_defined_biome_name` | `string` | UserDefinedBiomeName is a readable name of the biome that the player spawned in, such as 'plains'. This might be a custom biome name if any custom biomes are present through behaviour packs. |
| `dimension` | `int32` | Dimension is the ID of the dimension that the player spawns in. It is a value from 0-2, with 0 being the overworld, 1 being the nether and 2 being the end. |
| `generator` | `int32` | Generator is the generator used for the world. It is a value from 0-4, with 0 being old limited worlds, 1 being infinite worlds, 2 being flat worlds, 3 being nether worlds and 4 being end worlds. A value of 0 will actually make the client stop rendering chunks you send beyond the world limit. |
| `world_game_mode` | `int32` | WorldGameMode is the game mode that a player gets when it first spawns in the world. It is shown in the settings and is used if the PlayerGameMode is set to 5. |
| `hardcore` | `bool` | Hardcore is if the world is in hardcore mode. In hardcore mode, the player cannot respawn after dying. |
| `difficulty` | `int32` | Difficulty is the difficulty of the world. It is a value from 0-3, with 0 being peaceful, 1 being easy, 2 being normal and 3 being hard. |
| `world_spawn` | `protocol.BlockPos` | WorldSpawn is the block on which the world spawn of the world. This coordinate has no effect on the place that the client spawns, but it does have an effect on the direction that a compass points. |
| `achievements_disabled` | `bool` | AchievementsDisabled defines if achievements are disabled in the world. The client crashes if this value is set to true while the player's or the world's game mode is creative, and it's recommended to simply always set this to false as a server. |
| `editor_world_type` | `int32` | EditorWorldType is a value to dictate the type of editor mode, a special mode recently introduced adding "powerful tools for editing worlds, intended for experienced creators." It is one of the constants above. |
| `created_in_editor` | `bool` | CreatedInEditor is a value to dictate if the world was created as a project in the editor mode. The functionality of this field is currently unknown. |
| `exported_from_editor` | `bool` | ExportedFromEditor is a value to dictate if the world was exported from editor mode. The functionality of this field is currently unknown. |
| `day_cycle_lock_time` | `int32` | DayCycleLockTime is the time at which the day cycle was locked if the day cycle is disabled using the respective game rule. The client will maintain this time as long as the day cycle is disabled. |
| `education_edition_offer` | `int32` | EducationEditionOffer is some Minecraft: Education Edition field that specifies what 'region' the world was from, with 0 being None, 1 being RestOfWorld, and 2 being China. The actual use of this field is unknown. |
| `education_features_enabled` | `bool` | EducationFeaturesEnabled specifies if the world has education edition features enabled, such as the blocks or entities specific to education edition. |
| `education_product_id` | `string` | EducationProductID is a UUID used to identify the education edition server instance. It is generally unique for education edition servers. |
| `rain_level` | `float32` | RainLevel is the level specifying the intensity of the rain falling. When set to 0, no rain falls at all. |
| `lightning_level` | `float32` | LightningLevel is the level specifying the intensity of the thunder. This may actually be set independently from the RainLevel, meaning dark clouds can be produced without rain. |
| `confirmed_platform_locked_content` | `bool` | ConfirmedPlatformLockedContent ... |
| `multi_player_game` | `bool` | MultiPlayerGame specifies if the world is a multi-player game. This should always be set to true for servers. |
| `lanbroadcast_enabled` | `bool` | LANBroadcastEnabled specifies if LAN broadcast was intended to be enabled for the world. |
| `xblbroadcast_mode` | `int32` | XBLBroadcastMode is the mode used to broadcast the joined game across XBOX Live. |
| `platform_broadcast_mode` | `int32` | PlatformBroadcastMode is the mode used to broadcast the joined game across the platform. |
| `commands_enabled` | `bool` | CommandsEnabled specifies if commands are enabled for the player. It is recommended to always set this to true on the server, as setting it to false means the player cannot, under any circumstance, use a command. |
| `texture_pack_required` | `bool` | TexturePackRequired specifies if the texture pack the world might hold is required, meaning the client was forced to download it before joining. |
| `game_rules` | `[]protocol.GameRule` | GameRules defines game rules currently active with their respective values. The value of these game rules may be either 'bool', 'int32' or 'float32'. Some game rules are server side only, and don't necessarily need to be sent to the client. |
| `experiments` | `[]protocol.ExperimentData` | Experiments holds a list of experiments that are either enabled or disabled in the world that the player spawns in. |
| `experiments_previously_toggled` | `bool` | ExperimentsPreviouslyToggled specifies if any experiments were previously toggled in this world. It is probably used for some kind of metrics. |
| `bonus_chest_enabled` | `bool` | BonusChestEnabled specifies if the world had the bonus map setting enabled when generating it. It does not have any effect client-side. |
| `start_with_map_enabled` | `bool` | StartWithMapEnabled specifies if the world has the start with map setting enabled, meaning each joining player obtains a map. This should always be set to false, because the client obtains a map all on its own accord if this is set to true. |
| `player_permissions` | `int32` | PlayerPermissions is the permission level of the player. It is a value from 0-3, with 0 being visitor, 1 being member, 2 being operator and 3 being custom. |
| `server_chunk_tick_radius` | `int32` | ServerChunkTickRadius is the radius around the player in which chunks are ticked. Most servers set this value to a fixed number, as it does not necessarily affect anything client-side. |
| `has_locked_behaviour_pack` | `bool` | HasLockedBehaviourPack specifies if the behaviour pack of the world is locked, meaning it cannot be disabled from the world. This is typically set for worlds on the marketplace that have a dedicated behaviour pack. |
| `has_locked_texture_pack` | `bool` | HasLockedTexturePack specifies if the texture pack of the world is locked, meaning it cannot be disabled from the world. This is typically set for worlds on the marketplace that have a dedicated texture pack. |
| `from_locked_world_template` | `bool` | FromLockedWorldTemplate specifies if the world from the server was from a locked world template. For servers this should always be set to false. |
| `msagamer_tags_only` | `bool` | MSAGamerTagsOnly .. |
| `from_world_template` | `bool` | FromWorldTemplate specifies if the world from the server was from a world template. For servers this should always be set to false. |
| `world_template_settings_locked` | `bool` | WorldTemplateSettingsLocked specifies if the world was a template that locks all settings that change properties above in the settings GUI. It is recommended to set this to true for servers that do not allow things such as setting game rules through the GUI. |
| `only_spawn_v1_villagers` | `bool` | OnlySpawnV1Villagers is a hack that Mojang put in place to preserve backwards compatibility with old villagers. The bool is never actually read though, so it has no functionality. |
| `persona_disabled` | `bool` | PersonaDisabled is true if persona skins are disabled for the current game session. |
| `custom_skins_disabled` | `bool` | CustomSkinsDisabled is true if custom skins are disabled for the current game session. |
| `emote_chat_muted` | `bool` | EmoteChatMuted specifies if players will be sent a chat message when using certain emotes. |
| `base_game_version` | `string` | BaseGameVersion is the version of the game from which Vanilla features will be used. The exact function of this field isn't clear. |
| `limited_world_width` | `int32` |  |
| `limited_world_depth` | `int32` |  |
| `new_nether` | `bool` | NewNether specifies if the server runs with the new nether introduced in the 1.16 update. |
| `education_shared_resource_uri` | `protocol.EducationSharedResourceURI` | EducationSharedResourceURI is an education edition feature that transmits education resource settings to clients. |
| `force_experimental_gameplay` | `protocol.Optional[bool]` | ForceExperimentalGameplay specifies if experimental gameplay should be force enabled. For servers this should always be set to false. |
| `level_id` | `string` | LevelID is a base64 encoded world ID that is used to identify the world. |
| `world_name` | `string` | WorldName is the name of the world that the player is joining. Note that this field shows up above the player list for the rest of the game session, and cannot be changed. Setting the server name to this field is recommended. |
| `template_content_identity` | `string` | TemplateContentIdentity is a UUID specific to the premium world template that might have been used to generate the world. Servers should always fill out an empty string for this. |
| `trial` | `bool` | Trial specifies if the world was a trial world, meaning features are limited and there is a time limit on the world. |
| `player_movement_settings` | `protocol.PlayerMovementSettings` | PlayerMovementSettings ... |
| `time` | `int64` | Time is the total time that has elapsed since the start of the world. |
| `enchantment_seed` | `int32` | EnchantmentSeed is the seed used to seed the random used to produce enchantments in the enchantment table. Note that the exact correct random implementation must be used to produce the correct results both client- and server-side. |
| `blocks` | `[]protocol.BlockEntry` | Blocks is a list of all custom blocks registered on the server. |
| `multi_player_correlation_id` | `string` | MultiPlayerCorrelationID is a unique ID specifying the multi-player session of the player. A random UUID should be filled out for this field. |
| `server_authoritative_inventory` | `bool` | ServerAuthoritativeInventory specifies if the server authoritative inventory system is enabled. This is a new system introduced in 1.16. Backwards compatibility with the inventory transactions has to some extent been preserved, but will eventually be removed. |
| `game_version` | `string` | GameVersion is the version of the game the server is running. The exact function of this field isn't clear. |
| `property_data` | `map[string]interface {}` | PropertyData contains properties that should be applied on the player. These properties are the same as the ones that are sent in the SyncActorProperty packet. |
| `server_block_state_checksum` | `uint64` | ServerBlockStateChecksum is a checksum to ensure block states between the server and client match. This can simply be left empty, and the client will avoid trying to verify it. |
| `client_side_generation` | `bool` | ClientSideGeneration is true if the client should use the features registered in the FeatureRegistry packet to generate terrain client-side to save on bandwidth. |
| `world_template_id` | `uuid.UUID` | WorldTemplateID is a UUID that identifies the template that was used to generate the world. Servers that do not use a world based off of a template can set this to an empty UUID. |
| `chat_restriction_level` | `uint8` | ChatRestrictionLevel specifies the level of restriction on in-game chat. It is one of the constants above. |
| `disable_player_interactions` | `bool` | DisablePlayerInteractions is true if the client should ignore other players when interacting with the world. |
| `server_id` | `string` | ServerID is always empty in vanilla and its usage is currently unknown. |
| `world_id` | `string` | WorldID is always empty in vanilla and its usage is currently unknown. |
| `scenario_id` | `string` | ScenarioID is always empty in vanilla and its usage is currently unknown. |
| `owner_id` | `string` | OwnerID is always empty in vanilla and its usage is currently unknown. |
| `use_block_network_idhashes` | `bool` | UseBlockNetworkIDHashes is true if the client should use the hash of a block's name as its network ID rather than its index in the expected block palette. This is useful for servers that wish to support multiple protocol versions and custom blocks, but it will result in extra bytes being written for every block in a sub chunk palette. |
| `server_authoritative_sound` | `bool` | ServerAuthoritativeSound is currently unknown as to what it does. |

### game_rules 嵌套字段

- `name` (`string`)：Name is the name of the game rule.
- `can_be_modified_by_player` (`bool`)：CanBeModifiedByPlayer specifies if the game rule can be modified by the player through the in-game UI.
- `value` (`any`)：Value is the new value of the game rule. This is either a bool, uint32 or float32.

### experiments 嵌套字段

- `name` (`string`)：Name is the name of the experiment.
- `enabled` (`bool`)：Enabled specifies if the experiment is enabled. Vanilla typically always sets this to true for any experiments sent.

### education_shared_resource_uri 嵌套字段

- `button_name` (`string`)：ButtonName is the button name of the resource URI.
- `link_uri` (`string`)：LinkURI is the link URI for the resource URI.

### player_movement_settings 嵌套字段

- `movement_type` (`int32`)：MovementType is kept in-memory for legacy compatibility, but is no longer encoded in 1.21.90.
- `rewind_history_size` (`int32`)：RewindHistorySize is the amount of history to keep at maximum.
- `server_authoritative_block_breaking` (`bool`)：ServerAuthoritativeBlockBreaking specifies if block breaking should be sent through packet.PlayerAuthInput or not.

### blocks 嵌套字段

- `name` (`string`)：Name is the name of the custom block.
- `properties` (`map[string]any`)：Properties is a list of properties which, in combination with the name, specify a unique block.

## 12 · AddPlayer · 玩家实体进入视野

- 分组：实体状态
- 方向：服务器→客户端
- 用法与边界：用户名、UUID、runtime_id、坐标、速度、元数据、装备、能力与device_os以实际字段为准；设备ID可被客户端改变，不作可信身份。
- 本内核原始说明：AddPlayer is sent by the server to the client to make a player entity show up client-side. It is one of the few entities that cannot be sent using the AddActor packet.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `uuid` | `uuid.UUID` | UUID is the UUID of the player. It is the same UUID that the client sent in the Login packet at the start of the session. A player with this UUID must exist in the player list (built up using the PlayerList packet), for it to show up in-game. |
| `username` | `string` | Username is the name of the player. This username is the username that will be set as the initial name tag of the player. |
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the player. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `platform_chat_id` | `string` | PlatformChatID is an identifier only set for particular platforms when chatting (presumably only for Nintendo Switch). It is otherwise an empty string, and is used to decide which players are able to chat with each other. |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position to spawn the player on. If the player is on a distance that the viewer cannot see it, the player will still show up if the viewer moves closer. |
| `velocity` | `mgl32.Vec3` | 协议速度向量，单位需结合游戏刻核对。 Velocity is the initial velocity the player spawns with. This velocity will initiate client side movement of the player. |
| `pitch` | `float32` | 俯仰角。 Pitch is the vertical rotation of the player. Facing straight forward yields a pitch of 0. Pitch is measured in degrees. |
| `yaw` | `float32` | 偏航角。 Yaw is the horizontal rotation of the player. Yaw is also measured in degrees. |
| `head_yaw` | `float32` | 头部偏航角。 HeadYaw is the same as Yaw, except that it applies specifically to the head of the player. A different value for HeadYaw than Yaw means that the player will have its head turned. |
| `held_item` | `protocol.ItemInstance` | HeldItem is the item that the player is holding. The item is shown to the viewer as soon as the player itself shows up. Needless to say that this field is rather pointless, as additional packets still must be sent for armour to show up. |
| `game_type` | `int32` | GameType is the game type of the player. If set to GameTypeSpectator, the player will not be shown to viewers. |
| `entity_metadata` | `map[uint32]interface {}` | EntityMetadata is a map of entity metadata, which includes flags and data properties that alter in particular the way the player looks. Flags include ones such as 'on fire' and 'sprinting'. The metadata values are indexed by their property key. |
| `entity_properties` | `protocol.EntityProperties` | EntityProperties is a list of properties that the entity inhibits. These properties define and alter specific attributes of the entity. |
| `ability_data` | `protocol.AbilityData` | AbilityData represents various data about the abilities of a player, such as ability layers or permissions. |
| `entity_links` | `[]protocol.EntityLink` | EntityLinks is a list of entity links that are currently active on the player. These links alter the way the player shows up when first spawned in terms of it shown as riding an entity. Setting these links is important for new viewers to see the player is riding another entity. |
| `device_id` | `string` | DeviceID is the device ID set in one of the files found in the storage of the device of the player. It may be changed freely, so it should not be relied on for anything. |
| `build_platform` | `int32` | BuildPlatform is the build platform/device OS of the player that is about to be added, as it sent in the Login packet when joining. |

### held_item 嵌套字段

- `stack_network_id` (`int32`)：StackNetworkID is the network ID of the item stack. If the stack is empty, 0 is always written for this field. If not, the field should be set to 1 if the server authoritative inventories are disabled in the StartGame packet, or to a unique stack ID if it is enabled.
- `stack` (`ItemStack`)：Stack is the actual item stack of the item instance.
- `stack.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `stack.count` (`uint16`)：Count is the count of items that the item stack holds.
- `stack.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `stack.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `stack.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `stack.has_network_id` (`bool`)：HasNetworkID ...

### entity_properties 嵌套字段

- `integer_properties` (`[]IntegerEntityProperty`)：IntegerProperties is a list of entity properties that contain integer values.
- `integer_properties.index` (`uint32`)：Index represents the index of the property. It is unclear what the exact purpose of this is.
- `integer_properties.value` (`int32`)：Value is the value of the property.
- `float_properties` (`[]FloatEntityProperty`)：FloatProperties is a list of entity properties that contain float values.
- `float_properties.index` (`uint32`)：Index represents the index of the property. It is unclear what the exact purpose of this is.
- `float_properties.value` (`float32`)：Value is the value of the property.

### ability_data 嵌套字段

- `entity_unique_id` (`int64`)：EntityUniqueID is a unique identifier of the player. It appears it is not required to fill this field out with a correct value. Simply writing 0 seems to work.
- `player_permissions` (`byte`)：PlayerPermissions is the permission level of the player as it shows up in the player list built up using the PlayerList packet.
- `command_permissions` (`byte`)：CommandPermissions is a set of permissions that specify what commands a player is allowed to execute.
- `layers` (`[]AbilityLayer`)：Layers contains all ability layers and their potential values. This should at least have one entry, being the base layer.
- `layers.type` (`uint16`)：Type represents the type of the layer. This is one of the AbilityLayerType constants defined above.
- `layers.abilities` (`uint32`)：Abilities is a set of abilities that are enabled for the layer. This is one of the Ability constants defined above.
- `layers.values` (`uint32`)：Values is a set of values that are associated with the enabled abilities, representing the values of the abilities.
- `layers.fly_speed` (`float32`)：FlySpeed is the default horizontal fly speed of the layer.
- `layers.vertical_fly_speed` (`float32`)：VerticalFlySpeed is the default vertical fly speed of the layer.
- `layers.walk_speed` (`float32`)：WalkSpeed is the default walk speed of the layer.

### entity_links 嵌套字段

- `ridden_entity_unique_id` (`int64`)：RiddenEntityUniqueID is the entity unique ID of the entity that is being ridden. For a player sitting in a boat, this is the unique ID of the boat.
- `rider_entity_unique_id` (`int64`)：RiderEntityUniqueID is the entity unique ID of the entity that is riding. For a player sitting in a boat, this is the unique ID of the player.
- `type` (`byte`)：Type is one of the types above. It specifies the way the entity is linked to another entity.
- `immediate` (`bool`)：Immediate is set to immediately dismount an entity from another. This should be set when the mount of an entity is killed.
- `rider_initiated` (`bool`)：RiderInitiated specifies if the link was created by the rider, for example the player starting to ride a horse by itself. This is generally true in vanilla environment for players.
- `vehicle_angular_velocity` (`float32`)：VehicleAngularVelocity is the angular velocity of the vehicle that the rider is riding.

## 13 · AddActor · 普通实体进入视野

- 分组：实体状态
- 方向：服务器→客户端
- 用法与边界：可见实体类型/位置/速度/属性/元数据。属性中只有真正包含minecraft:health才知道血量；出现可能是进入视距。
- 本内核原始说明：AddActor is sent by the server to the client to spawn an entity to the player. It is used for every entity except other players, for which the AddPlayer packet is used.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_unique_id` | `int64` | 实体唯一ID，与运行ID不同。 EntityUniqueID is the unique ID of the entity. The unique ID is a value that remains consistent across different sessions of the same world, but most servers simply fill the runtime ID of the entity out for this field. |
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the entity. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `entity_type` | `string` | EntityType is the string entity type of the entity, for example 'minecraft:skeleton'. A list of these entities may be found online. |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position to spawn the entity on. If the entity is on a distance that the player cannot see it, the entity will still show up if the player moves closer. |
| `velocity` | `mgl32.Vec3` | 协议速度向量，单位需结合游戏刻核对。 Velocity is the initial velocity the entity spawns with. This velocity will initiate client side movement of the entity. |
| `pitch` | `float32` | 俯仰角。 Pitch is the vertical rotation of the entity. Facing straight forward yields a pitch of 0. Pitch is measured in degrees. |
| `yaw` | `float32` | 偏航角。 Yaw is the horizontal rotation of the entity. Yaw is also measured in degrees. |
| `head_yaw` | `float32` | 头部偏航角。 HeadYaw is the same as Yaw, except that it applies specifically to the head of the entity. A different value for HeadYaw than Yaw means that the entity will have its head turned. |
| `body_yaw` | `float32` | BodyYaw is the same as Yaw, except that it applies specifically to the body of the entity. A different value for BodyYaw than HeadYaw means that the entity will have its body turned, although it is unclear what the difference between BodyYaw and Yaw is. |
| `attributes` | `[]protocol.AttributeValue` | 实际同步属性列表，未下发的属性保持未知。 Attributes is a slice of attributes that the entity has. It includes attributes such as its health, movement speed, etc. |
| `entity_metadata` | `map[uint32]interface {}` | EntityMetadata is a map of entity metadata, which includes flags and data properties that alter in particular the way the entity looks. Flags include ones such as 'on fire' and 'sprinting'. The metadata values are indexed by their property key. |
| `entity_properties` | `protocol.EntityProperties` | EntityProperties is a list of properties that the entity inhibits. These properties define and alter specific attributes of the entity. |
| `entity_links` | `[]protocol.EntityLink` | EntityLinks is a list of entity links that are currently active on the entity. These links alter the way the entity shows up when first spawned in terms of it shown as riding an entity. Setting these links is important for new viewers to see the entity is riding another entity. |

### attributes 嵌套字段

- `name` (`string`)：Name is the name of the attribute, for example 'minecraft:health'. These names must be identical to the ones defined client-side.
- `value` (`float32`)：Value is the current value of the attribute. This value will be applied to the entity when sent in a packet.

### entity_properties 嵌套字段

- `integer_properties` (`[]IntegerEntityProperty`)：IntegerProperties is a list of entity properties that contain integer values.
- `integer_properties.index` (`uint32`)：Index represents the index of the property. It is unclear what the exact purpose of this is.
- `integer_properties.value` (`int32`)：Value is the value of the property.
- `float_properties` (`[]FloatEntityProperty`)：FloatProperties is a list of entity properties that contain float values.
- `float_properties.index` (`uint32`)：Index represents the index of the property. It is unclear what the exact purpose of this is.
- `float_properties.value` (`float32`)：Value is the value of the property.

### entity_links 嵌套字段

- `ridden_entity_unique_id` (`int64`)：RiddenEntityUniqueID is the entity unique ID of the entity that is being ridden. For a player sitting in a boat, this is the unique ID of the boat.
- `rider_entity_unique_id` (`int64`)：RiderEntityUniqueID is the entity unique ID of the entity that is riding. For a player sitting in a boat, this is the unique ID of the player.
- `type` (`byte`)：Type is one of the types above. It specifies the way the entity is linked to another entity.
- `immediate` (`bool`)：Immediate is set to immediately dismount an entity from another. This should be set when the mount of an entity is killed.
- `rider_initiated` (`bool`)：RiderInitiated specifies if the link was created by the rider, for example the player starting to ride a horse by itself. This is generally true in vanilla environment for players.
- `vehicle_angular_velocity` (`float32`)：VehicleAngularVelocity is the angular velocity of the vehicle that the rider is riding.

## 14 · RemoveActor · 实体移除或离开视野

- 分组：实体状态
- 方向：服务器→客户端
- 用法与边界：仅凭移除不能判断死亡、拾取者或击杀者；不可直接当死亡事件。
- 本内核原始说明：RemoveActor is sent by the server to remove an entity that currently exists in the world from the client- side. Sending this packet if the client cannot already see this entity will have no effect.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_unique_id` | `int64` | 实体唯一ID，与运行ID不同。 EntityUniqueID is the unique ID of the entity to be removed. The unique ID is a value that remains consistent across different sessions of the same world, but most servers simply fill the runtime ID of the entity out for this field. |

## 15 · AddItemActor · 掉落物进入视野

- 分组：实体状态
- 方向：服务器→客户端
- 用法与边界：保留物品Item、位置与RuntimeID，可与TakeItemActor匹配拾取；需要玩家靠近才有同步。
- 本内核原始说明：AddItemActor is sent by the server to the client to make an item entity show up. It is one of the few entities that cannot be sent using the AddActor packet

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_unique_id` | `int64` | 实体唯一ID，与运行ID不同。 EntityUniqueID is the unique ID of the entity. The unique ID is a value that remains consistent across different sessions of the same world, but most servers simply fill the runtime ID of the entity out for this field. |
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the entity. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `item` | `protocol.ItemInstance` | 物品栈，嵌套字段记录ID、数量与实际NBT。 Item is the item that is spawned. It must have a valid ID for it to show up client-side. If it is not a valid item, the client will crash when coming near. |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position to spawn the entity on. If the entity is on a distance that the player cannot see it, the entity will still show up if the player moves closer. |
| `velocity` | `mgl32.Vec3` | 协议速度向量，单位需结合游戏刻核对。 Velocity is the initial velocity the entity spawns with. This velocity will initiate client side movement of the entity. |
| `entity_metadata` | `map[uint32]interface {}` | EntityMetadata is a map of entity metadata, which includes flags and data properties that alter in particular the way the entity looks. Flags include ones such as 'on fire' and 'sprinting'. The metadata values are indexed by their property key. |
| `from_fishing` | `bool` | FromFishing specifies if the item was obtained by fishing it up using a fishing rod. It is not clear why the client needs to know this. |

### item 嵌套字段

- `stack_network_id` (`int32`)：StackNetworkID is the network ID of the item stack. If the stack is empty, 0 is always written for this field. If not, the field should be set to 1 if the server authoritative inventories are disabled in the StartGame packet, or to a unique stack ID if it is enabled.
- `stack` (`ItemStack`)：Stack is the actual item stack of the item instance.
- `stack.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `stack.count` (`uint16`)：Count is the count of items that the item stack holds.
- `stack.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `stack.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `stack.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `stack.has_network_id` (`bool`)：HasNetworkID ...

## 17 · TakeItemActor · 实体拾取掉落物

- 分组：实体状态
- 方向：服务器→客户端
- 用法与边界：明确提供掉落物和拾取者RuntimeID；可在真实拾取后奖励，但不能先假定拾取者一定为玩家。
- 本内核原始说明：TakeItemActor is sent by the server when a player picks up an item entity. It makes the item entity disappear to viewers and shows the pick-up animation.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `item_entity_runtime_id` | `uint64` | ItemEntityRuntimeID is the entity runtime ID of the item that is being taken by another entity. It will disappear to viewers after showing the pick-up animation. |
| `taker_entity_runtime_id` | `uint64` | TakerEntityRuntimeID is the runtime ID of the entity that took the item, which is usually a player, but could be another entity like a zombie too. |

## 18 · MoveActorAbsolute · 实体绝对位置

- 分组：移动与输入
- 方向：服务器→客户端
- 用法与边界：位置、朝向与标志，按实体RuntimeID维护观测。
- 本内核原始说明：MoveActorAbsolute is sent by the server to move an entity to an absolute position. It is typically used for movements where high accuracy isn't needed, such as for long range teleporting.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the entity. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `flags` | `uint8` | Flags is a combination of flags that specify details of the movement. It is a combination of the flags above. |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position to spawn the entity on. If the entity is on a distance that the player cannot see it, the entity will still show up if the player moves closer. |
| `rotation` | `mgl32.Vec3` | Rotation is a Vec3 holding the X, Y and Z rotation of the entity after the movement. This is a Vec3 for the reason that projectiles like arrows don't have yaw/pitch, but do have roll. |

## 19 · MovePlayer · 玩家位置与朝向

- 分组：移动与输入
- 方向：双向，具体模式由服务器决定
- 用法与边界：收到的坐标可测位移速度；瞬移/维度切换应重置速度采样。客户端提交位置不保证被采纳。
- 本内核原始说明：MovePlayer is sent by players to send their movement to the server, and by the server to update the movement of player entities to other players.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the player. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position to spawn the player on. If the player is on a distance that the viewer cannot see it, the player will still show up if the viewer moves closer. |
| `pitch` | `float32` | 俯仰角。 Pitch is the vertical rotation of the player. Facing straight forward yields a pitch of 0. Pitch is measured in degrees. |
| `yaw` | `float32` | 偏航角。 Yaw is the horizontal rotation of the player. Yaw is also measured in degrees. |
| `head_yaw` | `float32` | 头部偏航角。 HeadYaw is the same as Yaw, except that it applies specifically to the head of the player. A different value for HeadYaw than Yaw means that the player will have its head turned. |
| `mode` | `uint8` | Mode is the mode of the movement. It specifies the way the player's movement should be shown to other players. It is one of the constants above. |
| `on_ground` | `bool` | OnGround specifies if the player is considered on the ground. Note that proxies or hacked clients could fake this to always be true, so it should not be taken for granted. |
| `ridden_entity_runtime_id` | `uint64` | RiddenEntityRuntimeID is the runtime ID of the entity that the player might currently be riding. If not riding, this should be left 0. |
| `teleport_cause` | `int32` | TeleportCause is written only if Mode is MoveModeTeleport. It specifies the cause of the teleportation, which is one of the constants above. |
| `teleport_source_entity_type` | `int32` | TeleportSourceEntityType is the entity type that caused the teleportation, for example an ender pearl. |
| `tick` | `uint64` | 输入/同步的游戏刻编号。 Tick is the server tick at which the packet was sent. It is used in relation to CorrectPlayerMovePrediction. |

## 21 · UpdateBlock · 方块状态更新

- 分组：世界与结构
- 方向：服务器→客户端
- 用法与边界：position/runtime_id/layer；只能认定状态变化，不能单独确定是谁破坏。
- 本内核原始说明：UpdateBlock is sent by the server to update a block client-side, without resending the entire chunk that the block is located in. It is particularly useful for small modifications like block breaking/placing.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `position` | `protocol.BlockPos` | 位置坐标，维度与有效性按本包上下文确定。 Position is the block position at which a block is updated. |
| `new_block_runtime_id` | `uint32` | NewBlockRuntimeID is the runtime ID of the block that is placed at Position after sending the packet to the client. |
| `flags` | `uint32` | Flags is a combination of flags that specify the way the block is updated client-side. It is a combination of the flags above, but typically sending only the BlockUpdateNetwork flag is sufficient. |
| `layer` | `uint32` | Layer is the world layer on which the block is updated. For most blocks, this is the first layer, as that layer is the default layer to place blocks on, but for blocks inside of each other, this differs. |

## 22 · AddPainting · 绘画实体出现

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：绘画位置、朝向与图案，可用于可见装饰实体识别。
- 本内核原始说明：AddPainting is sent by the server to the client to make a painting entity show up. It is one of the few entities that cannot be sent using the AddActor packet.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_unique_id` | `int64` | 实体唯一ID，与运行ID不同。 EntityUniqueID is the unique ID of the entity. The unique ID is a value that remains consistent across different sessions of the same world, but most servers simply fill the runtime ID of the entity out for this field. |
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the entity. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position to spawn the entity on. If the entity is on a distance that the player cannot see it, the entity will still show up if the player moves closer. |
| `direction` | `int32` | Direction is the facing direction of the painting. |
| `title` | `string` | Title is the title of the painting. It specifies the motive of the painting. The title of the painting must be valid. |

## 25 · LevelEvent · 世界事件

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：LevelEvent is sent by the server to make a certain event in the level occur. It ranges from particles, to sounds, and other events such as starting rain and block breaking.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `event_type` | `int32` | EventType is the ID of the event that is being 'called'. It is one of the events found in the constants above. |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position of the level event. Practically every event requires this Vec3 set for it, as particles, sounds and block editing relies on it. |
| `event_data` | `int32` | EventData is an integer holding additional data of the event. The type of data held depends on the EventType. |

## 26 · BlockEvent · 方块事件

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：BlockEvent is sent by the server to initiate a certain event that has something to do with blocks in specific, for example opening a chest.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `position` | `protocol.BlockPos` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position of the block that an event occurred at. |
| `event_type` | `int32` | EventType is the type of the block event. The event type decides the way the event data that follows is used. It is one of the constants found above. |
| `event_data` | `int32` | EventData holds event type specific data. For chests for example, opening the chest means the data must hold 1, whereas closing it should hold 0. |

## 27 · ActorEvent · 实体动画与状态事件

- 分组：实体动作
- 方向：服务器→客户端（部分动作可请求）
- 用法与边界：死亡/受伤等事件通常无准确攻击者；不要凭受伤包编造谁杀了谁。
- 本内核原始说明：ActorEvent is sent by the server when a particular event happens that has to do with an entity. Some of these events are entity-specific, for example a wolf shaking itself dry, but others are used for each entity, such as dying.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the entity. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `event_type` | `uint8` | EventType is the ID of the event to be called. It is one of the constants that can be found above. |
| `event_data` | `int32` | EventData is optional data associated with a particular event. The data has a different function for different events, however most events don't use this field at all. |

## 28 · MobEffect · 实体药水效果

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：实体效果添加、更新或移除；只记录收到的效果。
- 本内核原始说明：MobEffect is sent by the server to apply an effect to the player, for example an effect like poison. It may also be used to modify existing effects, or removing them completely.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the entity. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `operation` | `uint8` | Operation is the operation of the packet. It is either MobEffectAdd, MobEffectModify or MobEffectRemove and specifies the result of the packet client-side. |
| `effect_type` | `int32` | EffectType is the ID of the effect to be added, removed or modified. It is one of the constants that may be found above. |
| `amplifier` | `int32` | Amplifier is the amplifier of the effect. Take note that the amplifier is not the same as the effect's level. The level is usually one higher than the amplifier, and the amplifier can actually be negative to reverse the behaviour effect. |
| `particles` | `bool` | Particles specifies if viewers of the entity that gets the effect shows particles around it. If set to false, no particles are emitted around the entity. |
| `duration` | `int32` | Duration is the duration of the effect in seconds. After the duration has elapsed, the effect will be removed automatically client-side. |
| `tick` | `uint64` | 输入/同步的游戏刻编号。 Tick is the server tick at which the packet was sent. It is used in relation to CorrectPlayerMovePrediction. |

## 29 · UpdateAttributes · 实体属性变化

- 分组：属性与状态
- 方向：服务器→客户端
- 用法与边界：属性数组含name/min/max/value/default等真实字段；可含血量、移动速度、攻击等。未同步的属性保持未知，不读出服务器未传的血量。
- 本内核原始说明：UpdateAttributes is sent by the server to update an amount of attributes of any entity in the world. These attributes include ones such as the health or the movement speed of the entity.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the entity. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `attributes` | `[]protocol.Attribute` | 实际同步属性列表，未下发的属性保持未知。 Attributes is a slice of new attributes that the entity gets. It includes attributes such as its health, movement speed, etc. Note that only changed attributes have to be sent in this packet. It is not required to send attributes that did not have their values changed. |
| `tick` | `uint64` | 输入/同步的游戏刻编号。 Tick is the server tick at which the packet was sent. It is used in relation to CorrectPlayerMovePrediction. |

### attributes 嵌套字段

- `default_min` (`float32`)：DefaultMin is the default minimum value of the attribute. It's not clear why this field must be sent to the client, but it is required regardless.
- `default_max` (`float32`)：DefaultMax is the default maximum value of the attribute. It's not clear why this field must be sent to the client, but it is required regardless.
- `default` (`float32`)：Default is the default value of the attribute. It's not clear why this field must be sent to the client, but it is required regardless.
- `modifiers` (`[]AttributeModifier`)：Modifiers is a slice of AttributeModifiers that are applied to the attribute.
- `modifiers.id` (`string`)：ID is the unique ID of the modifier. It is used to identify the modifier in the packet.
- `modifiers.name` (`string`)：Name is the name of the attribute that is modified.
- `modifiers.amount` (`float32`)：Amount is the amount of difference between the current value of the attribute and the new value.
- `modifiers.operation` (`int32`)：Operation is the operation that is performed on the attribute. It can be addition, multiply base, multiply total or cap.
- `modifiers.operand` (`int32`)：Operand ... TODO: Figure out what this field is used for.
- `modifiers.serializable` (`bool`)：Serializable ... TODO: Figure out what this field is used for.

## 30 · InventoryTransaction · 旧库存与交互事务

- 分组：物品与容器
- 方向：客户端→服务器
- 用法与边界：可含使用物品/攻击/交互分支。旁观端不自动看到他人的事务；联合类型请优先world.useBlock/mineBlock/bot.attack，服务器验证合法性。
- 本内核原始说明：InventoryTransaction is a packet sent by the client. It essentially exists out of multiple sub-packets, each of which have something to do with the inventory in one way or another. Some of these sub-packets directly relate to the inventory, others relate to interaction with the world, that could potentially result in a change in the inventory.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `legacy_request_id` | `int32` | LegacyRequestID is an ID that is only non-zero at times when sent by the client. The server should always send 0 for this. When this field is not 0, the LegacySetItemSlots slice below will have values in it. LegacyRequestID ties in with the ItemStackResponse packet. If this field is non-0, the server should respond with an ItemStackResponse packet. Some inventory actions such as dropping an item out of the hotbar are still one using this packet, and the ItemStackResponse packet needs to tie in with it. |
| `legacy_set_item_slots` | `[]protocol.LegacySetItemSlot` | LegacySetItemSlots are only present if the LegacyRequestID is non-zero. These item slots inform the server of the slots that were changed during the inventory transaction, and the server should send back an ItemStackResponse packet with these slots present in it. (Or false with no slots, if rejected.) |
| `actions` | `[]protocol.InventoryAction` | Actions is a list of actions that took place, that form the inventory transaction together. Each of these actions hold one slot in which one item was changed to another. In general, the combination of all of these actions results in a balanced inventory transaction. This should be checked to ensure that no items are cheated into the inventory. |
| `transaction_data` | `protocol.InventoryTransactionData` | TransactionData is a data object that holds data specific to the type of transaction that the TransactionPacket held. Its concrete type must be one of NormalTransactionData, MismatchTransactionData UseItemTransactionData, UseItemOnEntityTransactionData or ReleaseItemTransactionData. If nil is set, the transaction will be assumed to of type InventoryTransactionTypeNormal. |

### legacy_set_item_slots 嵌套字段

- `container_id` (`byte`)：
- `slots` (`[]byte`)：

### actions 嵌套字段

- `source_type` (`uint32`)：SourceType is the source type of the inventory action. It is one of the constants above.
- `window_id` (`int32`)：WindowID is the ID of the window that the client has opened. The window ID is not set if the SourceType is InventoryActionSourceWorld.
- `source_flags` (`uint32`)：SourceFlags is a combination of flags that is only set if the SourceType is InventoryActionSourceWorld.
- `inventory_slot` (`uint32`)：InventorySlot is the slot in which the action took place. Each action only describes the change of item in a single slot.
- `old_item` (`ItemInstance`)：OldItem is the item that was present in the slot before the inventory action. It should be checked by the server to ensure the inventories were not out of sync.
- `old_item.stack_network_id` (`int32`)：StackNetworkID is the network ID of the item stack. If the stack is empty, 0 is always written for this field. If not, the field should be set to 1 if the server authoritative inventories are disabled in the StartGame packet, or to a unique stack ID if it is enabled.
- `old_item.stack` (`ItemStack`)：Stack is the actual item stack of the item instance.
- `old_item.stack.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `old_item.stack.count` (`uint16`)：Count is the count of items that the item stack holds.
- `old_item.stack.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `old_item.stack.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `old_item.stack.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `old_item.stack.has_network_id` (`bool`)：HasNetworkID ...
- `new_item` (`ItemInstance`)：NewItem is the new item that was put in the InventorySlot that the OldItem was in. It must be checked in combination with other inventory actions to ensure that the transaction is balanced.
- `new_item.stack_network_id` (`int32`)：StackNetworkID is the network ID of the item stack. If the stack is empty, 0 is always written for this field. If not, the field should be set to 1 if the server authoritative inventories are disabled in the StartGame packet, or to a unique stack ID if it is enabled.
- `new_item.stack` (`ItemStack`)：Stack is the actual item stack of the item instance.
- `new_item.stack.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `new_item.stack.count` (`uint16`)：Count is the count of items that the item stack holds.
- `new_item.stack.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `new_item.stack.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `new_item.stack.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `new_item.stack.has_network_id` (`bool`)：HasNetworkID ...

## 31 · MobEquipment · 实体手持物与槽位

- 分组：物品与容器
- 方向：双向，服务端广播可见装备
- 用法与边界：Item含网络ID/count/NBT等实际类型；命名、附魔和特殊值需从真实NBT提取，不保证有完整其他玩家背包。
- 本内核原始说明：MobEquipment is sent by the client to the server and the server to the client to make the other side aware of the new item that an entity is holding. It is used to show the item in the hand of entities such as zombies too.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the entity. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `new_item` | `protocol.ItemInstance` | NewItem is the new item held after sending the MobEquipment packet. The entity will be shown holding that item to the player it was sent to. |
| `inventory_slot` | `uint8` | InventorySlot is the slot in the inventory that was held. This is the same as HotBarSlot, and only remains for backwards compatibility. |
| `hot_bar_slot` | `uint8` | HotBarSlot is the slot in the hot bar that was held. It is the same as InventorySlot, which is only there for backwards compatibility purposes. |
| `window_id` | `uint8` | 当前容器窗口ID。 WindowID is the window ID of the window that had its equipped item changed. This is usually the window ID of the normal inventory, but may also be something else, for example with the off hand. |

### new_item 嵌套字段

- `stack_network_id` (`int32`)：StackNetworkID is the network ID of the item stack. If the stack is empty, 0 is always written for this field. If not, the field should be set to 1 if the server authoritative inventories are disabled in the StartGame packet, or to a unique stack ID if it is enabled.
- `stack` (`ItemStack`)：Stack is the actual item stack of the item instance.
- `stack.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `stack.count` (`uint16`)：Count is the count of items that the item stack holds.
- `stack.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `stack.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `stack.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `stack.has_network_id` (`bool`)：HasNetworkID ...

## 32 · MobArmourEquipment · 实体穿戴装备

- 分组：物品与容器
- 方向：服务器→客户端
- 用法与边界：可以按实际物品与NBT识别专属装备；只收到服务器同步的穿戴槽，不是全部背包。
- 本内核原始说明：MobArmourEquipment is sent by the server to the client to update the armour an entity is wearing. It is sent for both players and other entities, such as zombies.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the entity. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `helmet` | `protocol.ItemInstance` | Helmet is the equipped helmet of the entity. Items that are not wearable on the head will not be rendered by the client. Unlike in Java Edition, blocks cannot be worn. |
| `chestplate` | `protocol.ItemInstance` | Chestplate is the chestplate of the entity. Items that are not wearable as chestplate will not be rendered. |
| `leggings` | `protocol.ItemInstance` | Leggings is the item worn as leggings by the entity. Items not wearable as leggings will not be rendered client-side. |
| `boots` | `protocol.ItemInstance` | Boots is the item worn as boots by the entity. Items not wearable as boots will not be rendered. |
| `body` | `protocol.ItemInstance` | Body is the item worn on the body of the entity. Items not wearable on the body will not be rendered. |

### helmet 嵌套字段

- `stack_network_id` (`int32`)：StackNetworkID is the network ID of the item stack. If the stack is empty, 0 is always written for this field. If not, the field should be set to 1 if the server authoritative inventories are disabled in the StartGame packet, or to a unique stack ID if it is enabled.
- `stack` (`ItemStack`)：Stack is the actual item stack of the item instance.
- `stack.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `stack.count` (`uint16`)：Count is the count of items that the item stack holds.
- `stack.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `stack.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `stack.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `stack.has_network_id` (`bool`)：HasNetworkID ...

### chestplate 嵌套字段

- `stack_network_id` (`int32`)：StackNetworkID is the network ID of the item stack. If the stack is empty, 0 is always written for this field. If not, the field should be set to 1 if the server authoritative inventories are disabled in the StartGame packet, or to a unique stack ID if it is enabled.
- `stack` (`ItemStack`)：Stack is the actual item stack of the item instance.
- `stack.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `stack.count` (`uint16`)：Count is the count of items that the item stack holds.
- `stack.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `stack.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `stack.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `stack.has_network_id` (`bool`)：HasNetworkID ...

### leggings 嵌套字段

- `stack_network_id` (`int32`)：StackNetworkID is the network ID of the item stack. If the stack is empty, 0 is always written for this field. If not, the field should be set to 1 if the server authoritative inventories are disabled in the StartGame packet, or to a unique stack ID if it is enabled.
- `stack` (`ItemStack`)：Stack is the actual item stack of the item instance.
- `stack.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `stack.count` (`uint16`)：Count is the count of items that the item stack holds.
- `stack.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `stack.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `stack.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `stack.has_network_id` (`bool`)：HasNetworkID ...

### boots 嵌套字段

- `stack_network_id` (`int32`)：StackNetworkID is the network ID of the item stack. If the stack is empty, 0 is always written for this field. If not, the field should be set to 1 if the server authoritative inventories are disabled in the StartGame packet, or to a unique stack ID if it is enabled.
- `stack` (`ItemStack`)：Stack is the actual item stack of the item instance.
- `stack.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `stack.count` (`uint16`)：Count is the count of items that the item stack holds.
- `stack.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `stack.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `stack.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `stack.has_network_id` (`bool`)：HasNetworkID ...

### body 嵌套字段

- `stack_network_id` (`int32`)：StackNetworkID is the network ID of the item stack. If the stack is empty, 0 is always written for this field. If not, the field should be set to 1 if the server authoritative inventories are disabled in the StartGame packet, or to a unique stack ID if it is enabled.
- `stack` (`ItemStack`)：Stack is the actual item stack of the item instance.
- `stack.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `stack.count` (`uint16`)：Count is the count of items that the item stack holds.
- `stack.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `stack.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `stack.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `stack.has_network_id` (`bool`)：HasNetworkID ...

## 33 · Interact · 交互

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：Interact is sent by the client when it interacts with another entity in some way. It used to be used for normal entity and block interaction, but this is no longer the case now.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `action_type` | `uint8` | 动作枚举，具体编号使用本内核定义。 Action type is the ID of the action that was executed by the player. It is one of the constants that may be found above. |
| `target_entity_runtime_id` | `uint64` | TargetEntityRuntimeID is the runtime ID of the entity that the player interacted with. This is empty for the InteractActionOpenInventory action type. |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position associated with the ActionType above. For the InteractActionMouseOverEntity, this is the position relative to the entity moused over over which the player hovered with its mouse/touch. For the InteractActionLeaveVehicle, this is the position that the player spawns at after leaving the vehicle. |

## 34 · BlockPickRequest · 方块选取请求

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：BlockPickRequest is sent by the client when it requests to pick a block in the world and place its item in their inventory.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `position` | `protocol.BlockPos` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position at which the client requested to pick the block. The block at that position should have its item put in HotBarSlot if it is empty. |
| `add_block_nbt` | `bool` | AddBlockNBT specifies if the item should get all NBT tags from the block, meaning the item places a block practically always equal to the one picked. |
| `hot_bar_slot` | `uint8` | HotBarSlot is the slot that was held at the time of picking a block. |

## 35 · ActorPickRequest · 实体选取请求

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ActorPickRequest is sent by the client when it tries to pick an entity, so that it gets a spawn egg which can spawn that entity.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_unique_id` | `int64` | 实体唯一ID，与运行ID不同。 EntityUniqueID is the unique ID of the entity that was attempted to be picked. The server must find the type of that entity and provide the correct spawn egg to the player. |
| `hot_bar_slot` | `uint8` | HotBarSlot is the held hot bar slot of the player at the time of trying to pick the entity. If empty, the resulting spawn egg should be put into this slot. |
| `with_data` | `bool` | WithData is true if the pick request requests the entity metadata. |

## 36 · PlayerAction · 本机玩家动作请求

- 分组：移动与输入
- 方向：客户端→服务器（部分服务端响应）
- 用法与边界：开始/停止挖掘等动作请求，需要合法距离、物品和服务器确认；不代表可以监听所有其他玩家的按键。
- 本内核原始说明：PlayerAction is sent by the client when it executes any action, for example starting to sprint, swim, starting the breaking of a block, dropping an item, etc.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the player. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `action_type` | `int32` | 动作枚举，具体编号使用本内核定义。 ActionType is the ID of the action that was executed by the player. It is one of the constants that may be found in protocol/player.go. |
| `block_position` | `protocol.BlockPos` | BlockPosition is the position of the target block, if the action with the ActionType set concerned a block. If that is not the case, the block position will be zero. |
| `result_position` | `protocol.BlockPos` | ResultPosition is the position of the action's result. When a UseItemOn action is sent, this is the position of the block clicked, but when a block is placed, this is the position at which the block will be placed. |
| `block_face` | `int32` | BlockFace is the face of the target block that was touched. If the action with the ActionType set concerned a block. If not, the face is always 0. |

## 38 · HurtArmour · 受伤护甲

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：HurtArmour is sent by the server to damage the player's armour after being hit. The packet should never be used by servers as it hands the responsibility over to the player completely, while the server can easily reliably update the armour damage of players itself.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `cause` | `int32` | Cause is the cause of the damage dealt to the armour. |
| `damage` | `int32` | Damage is the amount of damage points that was dealt to the player. The damage to the armour will be calculated by the client based upon this damage, and will also be based upon any enchantments like thorns that the armour may have. |
| `armour_slots` | `int64` | ArmourSlots is a bitset of all armour slots affected. |

## 39 · SetActorData · 设置实体数据

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：SetActorData is sent by the server to update the entity metadata of an entity. It includes flags such as if the entity is on fire, but also properties such as the air it has left until it starts drowning.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the entity. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `entity_metadata` | `map[uint32]interface {}` | EntityMetadata is a map of entity metadata, which includes flags and data properties that alter in particular the way the entity looks. Flags include ones such as 'on fire' and 'sprinting'. The metadata values are indexed by their property key. |
| `entity_properties` | `protocol.EntityProperties` | EntityProperties is a list of properties that the entity inhibits. These properties define and alter specific attributes of the entity. |
| `tick` | `uint64` | 输入/同步的游戏刻编号。 Tick is the server tick at which the packet was sent. It is used in relation to CorrectPlayerMovePrediction. |

### entity_properties 嵌套字段

- `integer_properties` (`[]IntegerEntityProperty`)：IntegerProperties is a list of entity properties that contain integer values.
- `integer_properties.index` (`uint32`)：Index represents the index of the property. It is unclear what the exact purpose of this is.
- `integer_properties.value` (`int32`)：Value is the value of the property.
- `float_properties` (`[]FloatEntityProperty`)：FloatProperties is a list of entity properties that contain float values.
- `float_properties.index` (`uint32`)：Index represents the index of the property. It is unclear what the exact purpose of this is.
- `float_properties.value` (`float32`)：Value is the value of the property.

## 40 · SetActorMotion · 实体速度向量

- 分组：移动与输入
- 方向：服务器→客户端
- 用法与边界：速度向量是协议值，不默认单位为米/秒；需核对游戏tick和同步方式再算仪表盘。
- 本内核原始说明：SetActorMotion is sent by the server to change the client-side velocity of an entity. It is usually used in combination with server-side movement calculation.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the entity. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `velocity` | `mgl32.Vec3` | 协议速度向量，单位需结合游戏刻核对。 Velocity is the new velocity the entity gets. This velocity will initiate the client-side movement of the entity. |
| `tick` | `uint64` | 输入/同步的游戏刻编号。 Tick is the server tick at which the packet was sent. It is used in relation to CorrectPlayerMovePrediction. |

## 41 · SetActorLink · 设置实体关联

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：SetActorLink is sent by the server to initiate an entity link client-side, meaning one entity will start riding another.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_link` | `protocol.EntityLink` | EntityLink is the link to be set client-side. It links two entities together, so that one entity rides another. Note that players that see those entities later will not see the link, unless it is also sent in the AddActor and AddPlayer packets. |

### entity_link 嵌套字段

- `ridden_entity_unique_id` (`int64`)：RiddenEntityUniqueID is the entity unique ID of the entity that is being ridden. For a player sitting in a boat, this is the unique ID of the boat.
- `rider_entity_unique_id` (`int64`)：RiderEntityUniqueID is the entity unique ID of the entity that is riding. For a player sitting in a boat, this is the unique ID of the player.
- `type` (`byte`)：Type is one of the types above. It specifies the way the entity is linked to another entity.
- `immediate` (`bool`)：Immediate is set to immediately dismount an entity from another. This should be set when the mount of an entity is killed.
- `rider_initiated` (`bool`)：RiderInitiated specifies if the link was created by the rider, for example the player starting to ride a horse by itself. This is generally true in vanilla environment for players.
- `vehicle_angular_velocity` (`float32`)：VehicleAngularVelocity is the angular velocity of the vehicle that the rider is riding.

## 42 · SetHealth · 本机玩家血量

- 分组：属性与状态
- 方向：服务器→本机客户端
- 用法与边界：主要更新本机器人血量，不能拿它当全服玩家血量。
- 本内核原始说明：SetHealth is sent by the server. It sets the health of the player it is sent to. The SetHealth packet should no longer be used. Instead, the health attribute should be used so that the health and maximum health may be changed directly.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `health` | `int32` | Health is the new health of the player. |

## 43 · SetSpawnPosition · 设置出生位置

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：SetSpawnPosition is sent by the server to update the spawn position of a player, for example when sleeping in a bed.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `spawn_type` | `int32` | SpawnType is the type of spawn to set. It is either SpawnTypePlayer or SpawnTypeWorld, and specifies the behaviour of the spawn set. If SpawnTypeWorld is set, the position to which compasses will point is also changed. |
| `position` | `protocol.BlockPos` | 位置坐标，维度与有效性按本包上下文确定。 Position is the new position of the spawn that was set. If SpawnType is SpawnTypeWorld, compasses will point to this position. As of 1.16, Position is always the position of the player. |
| `dimension` | `int32` | Dimension is the ID of the dimension that had its spawn updated. This is specifically relevant for behaviour added in 1.16 such as the respawn anchor, which allows setting the spawn in a specific dimension. |
| `spawn_position` | `protocol.BlockPos` | SpawnPosition is a new field added in 1.16. It holds the spawn position of the world. This spawn position is {-2147483648, -2147483648, -2147483648} for a default spawn position. |

## 44 · Animate · 挥臂与暴击动画

- 分组：实体动作
- 方向：服务器→观察客户端（也可提交自身动画请求）
- 用法与边界：action_type=1通常是挥臂。能做挥臂触发技能，不能证明打中了谁。优先events.onSwing。
- 本内核原始说明：Animate is sent by the server to send a player animation from one player to all viewers of that player. It is used for a couple of actions, such as arm swimming and critical hits.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `action_type` | `int32` | 动作枚举，具体编号使用本内核定义。 ActionType is the ID of the animation action to execute. It is one of the action type constants that may be found above. |
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the player that the animation should be played upon. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `boat_rowing_time` | `float32` | BoatRowingTime ... |

## 45 · Respawn · 重生状态同步

- 分组：属性与状态
- 方向：双向
- 用法与边界：处理ready状态，旧血量失效后等待新属性；不因重生包推算满血。
- 本内核原始说明：Respawn is sent by the server to make a player respawn client-side. It is sent in response to a PlayerAction packet with ActionType PlayerActionRespawn. As of 1.13, the server sends two of these packets with different states, and the client sends one of these back in order to complete the respawn.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position on which the player should be respawned. The position might be in a different dimension, in which case the client should first be sent a ChangeDimension packet. |
| `state` | `uint8` | State is the 'state' of the respawn. It is one of the constants that may be found above, and the value the packet contains depends on whether the server or client sends it. |
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the entity runtime ID of the player that the respawn packet concerns. This is apparently for the server to recognise which player sends this packet. |

## 46 · ContainerOpen · 本机打开容器

- 分组：物品与容器
- 方向：服务器→本机客户端
- 用法与边界：容器ID/类型/位置与实体ID；打开他人的私有箱子需服务端允许。
- 本内核原始说明：ContainerOpen is sent by the server to open a container client-side. This container must be physically present in the world, for the packet to have any effect. Unlike Java Edition, Bedrock Edition requires that chests for example must be present and in range to open its inventory.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `window_id` | `uint8` | 当前容器窗口ID。 WindowID is the ID representing the window that is being opened. It may be used later to close the container using a ContainerClose packet. |
| `container_type` | `uint8` | ContainerType is the type ID of the container that is being opened when opening the container at the position of the packet. It depends on the block/entity, and could, for example, be the window type of a chest or a hopper, but also a horse inventory. |
| `container_position` | `protocol.BlockPos` | ContainerPosition is the position of the container opened. The position must point to a block entity that actually has a container. If that is not the case, the window will not be opened and the packet will be ignored, if a valid ContainerEntityUniqueID has not also been provided. |
| `container_entity_unique_id` | `int64` | ContainerEntityUniqueID is the unique ID of the entity container that was opened. It is only used if the ContainerType is one that points to an entity, for example a horse. |

## 47 · ContainerClose · 关闭容器

- 分组：物品与容器
- 方向：双向
- 用法与边界：关闭当前本机窗口；不能任意关闭别人的容器。
- 本内核原始说明：ContainerClose is sent by the server to close a container the player currently has opened, which was opened using the ContainerOpen packet, or by the client to tell the server it closed a particular container, such as the crafting grid.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `window_id` | `uint8` | 当前容器窗口ID。 WindowID is the ID representing the window of the container that should be closed. It must be equal to the one sent in the ContainerOpen packet to close the designated window. |
| `container_type` | `uint8` | ContainerType is the type of container that the server is trying to close. This is used to validate on the client side whether or not the server's close request is valid. |
| `server_side` | `bool` | ServerSide determines whether or not the container was force-closed by the server. If this value is not set correctly, the client may ignore the packet and respond with a PacketViolationWarning. |

## 48 · PlayerHotBar · 玩家快捷栏

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：PlayerHotBar is sent by the server to the client. It used to be used to link hot bar slots of the player to actual slots in the inventory, but as of 1.2, this was changed and hot bar slots are no longer a free floating part of the inventory. Since 1.2, the packet has been re-purposed, but its new functionality is not clear.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `selected_hot_bar_slot` | `uint32` | SelectedHotBarSlot ... |
| `window_id` | `uint8` | 当前容器窗口ID。 WindowID ... |
| `select_hot_bar_slot` | `bool` | SelectHotBarSlot ... |

## 49 · InventoryContent · 容器完整内容

- 分组：物品与容器
- 方向：服务器→本机客户端
- 用法与边界：收到的容器内容，可实现自己已打开容器的菜单观测；不是所有玩家所有箱子。
- 本内核原始说明：InventoryContent is sent by the server to update the full content of a particular inventory. It is usually sent for the main inventory of the player, but also works for other inventories that are currently opened by the player.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `window_id` | `uint32` | 当前容器窗口ID。 WindowID is the ID that identifies one of the windows that the client currently has opened, or one of the consistent windows such as the main inventory. |
| `content` | `[]protocol.ItemInstance` | Content is the new content of the inventory. The length of this slice must be equal to the full size of the inventory window updated. |
| `container` | `protocol.FullContainerName` | Container is the protocol.FullContainerName that describes the container that the content is for. |
| `storage_item` | `protocol.ItemInstance` | StorageItem is the item that is acting as the storage container for the inventory. If the inventory is not a dynamic container then this field should be left empty. When set, only the item type is used by the client and none of the other stack info. |

### content 嵌套字段

- `stack_network_id` (`int32`)：StackNetworkID is the network ID of the item stack. If the stack is empty, 0 is always written for this field. If not, the field should be set to 1 if the server authoritative inventories are disabled in the StartGame packet, or to a unique stack ID if it is enabled.
- `stack` (`ItemStack`)：Stack is the actual item stack of the item instance.
- `stack.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `stack.count` (`uint16`)：Count is the count of items that the item stack holds.
- `stack.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `stack.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `stack.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `stack.has_network_id` (`bool`)：HasNetworkID ...

### container 嵌套字段

- `container_id` (`byte`)：ContainerID is the ID of the container that the slot was in.
- `dynamic_container_id` (`Optional[uint32]`)：DynamicContainerID is the ID of the container if it is dynamic. If the container is not dynamic, this field should be left empty. A non-optional value of 0 is assumed to be non-empty.

### storage_item 嵌套字段

- `stack_network_id` (`int32`)：StackNetworkID is the network ID of the item stack. If the stack is empty, 0 is always written for this field. If not, the field should be set to 1 if the server authoritative inventories are disabled in the StartGame packet, or to a unique stack ID if it is enabled.
- `stack` (`ItemStack`)：Stack is the actual item stack of the item instance.
- `stack.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `stack.count` (`uint16`)：Count is the count of items that the item stack holds.
- `stack.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `stack.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `stack.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `stack.has_network_id` (`bool`)：HasNetworkID ...

## 50 · InventorySlot · 单槽物品更新

- 分组：物品与容器
- 方向：服务器→本机客户端
- 用法与边界：window/slot和item更新，只能统计已收到的本机库存，不能据此保证查出所有刷物品行为。
- 本内核原始说明：InventorySlot is sent by the server to update a single slot in one of the inventory windows that the client currently has opened. Usually this is the main inventory, but it may also be the off hand or, for example, a chest inventory.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `window_id` | `uint32` | 当前容器窗口ID。 WindowID is the ID of the window that the packet modifies. It must point to one of the windows that the client currently has opened. |
| `slot` | `uint32` | 槽位编号。 Slot is the index of the slot that the packet modifies. The new item will be set to the slot at this index. |
| `container` | `protocol.FullContainerName` | Container is the protocol.FullContainerName that describes the container that the content is for. |
| `storage_item` | `protocol.ItemInstance` | StorageItem is the item that is acting as the storage container for the inventory. If the inventory is not a dynamic container then this field should be left empty. When set, only the item type is used by the client and none of the other stack info. |
| `new_item` | `protocol.ItemInstance` | NewItem is the item to be put in the slot at Slot. It will overwrite any item that may currently be present in that slot. |

### container 嵌套字段

- `container_id` (`byte`)：ContainerID is the ID of the container that the slot was in.
- `dynamic_container_id` (`Optional[uint32]`)：DynamicContainerID is the ID of the container if it is dynamic. If the container is not dynamic, this field should be left empty. A non-optional value of 0 is assumed to be non-empty.

### storage_item 嵌套字段

- `stack_network_id` (`int32`)：StackNetworkID is the network ID of the item stack. If the stack is empty, 0 is always written for this field. If not, the field should be set to 1 if the server authoritative inventories are disabled in the StartGame packet, or to a unique stack ID if it is enabled.
- `stack` (`ItemStack`)：Stack is the actual item stack of the item instance.
- `stack.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `stack.count` (`uint16`)：Count is the count of items that the item stack holds.
- `stack.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `stack.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `stack.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `stack.has_network_id` (`bool`)：HasNetworkID ...

### new_item 嵌套字段

- `stack_network_id` (`int32`)：StackNetworkID is the network ID of the item stack. If the stack is empty, 0 is always written for this field. If not, the field should be set to 1 if the server authoritative inventories are disabled in the StartGame packet, or to a unique stack ID if it is enabled.
- `stack` (`ItemStack`)：Stack is the actual item stack of the item instance.
- `stack.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `stack.count` (`uint16`)：Count is the count of items that the item stack holds.
- `stack.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `stack.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `stack.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `stack.has_network_id` (`bool`)：HasNetworkID ...

## 51 · ContainerSetData · 容器设置数据

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ContainerSetData is sent by the server to update specific data of a single container, meaning a block such as a furnace or a brewing stand. This data is usually used by the client to display certain features client-side.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `window_id` | `uint8` | 当前容器窗口ID。 WindowID is the ID of the window that should have its data set. The player must have a window open with the window ID passed, or nothing will happen. |
| `key` | `int32` | Key is the key of the property. It is one of the constants that can be found above. Multiple properties share the same key, but the functionality depends on the type of the container that the data is set to. |
| `value` | `int32` | Value is the value of the property. Its use differs per property. |

## 52 · CraftingData · 合成数据

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：CraftingData is sent by the server to let the client know all crafting data that the server maintains. This includes shapeless crafting, crafting table recipes, furnace recipes etc. Each crafting station's recipes are included in it.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `recipes` | `[]protocol.Recipe` | Recipes is a list of all recipes available on the server. It includes among others shapeless, shaped and furnace recipes. The client will only be able to craft these recipes. |
| `potion_recipes` | `[]protocol.PotionRecipe` | PotionRecipes is a list of all potion mixing recipes which may be used in the brewing stand. |
| `potion_container_change_recipes` | `[]protocol.PotionContainerChangeRecipe` | PotionContainerChangeRecipes is a list of all recipes to convert a potion from one type to another, such as from a drinkable potion to a splash potion, or from a splash potion to a lingering potion. |
| `material_reducers` | `[]protocol.MaterialReducer` | MaterialReducers is a list of all material reducers which is used in education edition chemistry. |
| `clear_recipes` | `bool` | ClearRecipes indicates if all recipes currently active on the client should be cleaned. Doing this means that the client will have no recipes active by itself: Any CraftingData packets previously sent will also be discarded, and only the recipes in this CraftingData packet will be used. |

### potion_recipes 嵌套字段

- `input_potion_id` (`int32`)：InputPotionID is the item ID of the potion to be put in.
- `input_potion_metadata` (`int32`)：InputPotionMetadata is the type of the potion to be put in. This is typically the meta of the awkward potion (or water bottle to create an awkward potion).
- `reagent_item_id` (`int32`)：ReagentItemID is the item ID of the item that needs to be added to the brewing stand in order to brew the output potion.
- `reagent_item_metadata` (`int32`)：ReagentItemMetadata is the metadata value of the item that needs to be added to the brewing stand in order to brew the output potion.
- `output_potion_id` (`int32`)：OutputPotionID is the item ID of the potion obtained as a result of the brewing recipe.
- `output_potion_metadata` (`int32`)：OutputPotionMetadata is the type of the potion that is obtained as a result of brewing the input potion with the reagent item.

### potion_container_change_recipes 嵌套字段

- `input_item_id` (`int32`)：InputItemID is the item ID of the item to be put in. This is typically either the ID of a normal potion or a splash potion.
- `reagent_item_id` (`int32`)：ReagentItemID is the item ID of the item that needs to be added to the container in order to create the output item.
- `output_item_id` (`int32`)：OutputItemID is the item that is created using a combination of the InputItem and ReagentItem, which is typically either the ID of a splash potion or a lingering potion.

### material_reducers 嵌套字段

- `input_item` (`ItemType`)：InputItem is the starting item.
- `input_item.network_id` (`int32`)：NetworkID is the numerical network ID of the item. This is sometimes a positive ID, and sometimes a negative ID, depending on what item it concerns.
- `input_item.metadata_value` (`uint32`)：MetadataValue is the metadata value of the item. For some items, this is the damage value, whereas for other items it is simply an identifier of a variant of the item.
- `outputs` (`[]MaterialReducerOutput`)：Outputs contain all outputting items.
- `outputs.network_id` (`int32`)：NetworkID is the network ID of the output.
- `outputs.count` (`int32`)：Count is the quantity of the output.

## 54 · GUIDataPickItem · 界面数据选取物品

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：GUIDataPickItem is sent by the server to make the client 'select' a hot bar slot. It currently appears to be broken however, and does not actually set the selected slot to the hot bar slot set in the packet.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `item_name` | `string` | ItemName is the name of the item that shows up in the top part of the popup that shows up when selecting an item. It is shown as if an item was selected by the player itself. |
| `item_effects` | `string` | ItemEffects is the line under the ItemName, where the effects of the item are usually situated. |
| `hot_bar_slot` | `int32` | HotBarSlot is the hot bar slot to be selected/picked. This does not currently work, so it does not matter what number this is. |

## 55 · AdventureSettings · 冒险模式与权限设置

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：旧协议的权限和能力位，不能靠修改包取得服务器权限。
- 本内核原始说明：AdventureSettings is sent by the server to update game-play related features, in particular permissions to access these features for the client. It includes allowing the player to fly, build and mine, and attack entities. Most of these flags should be checked server-side instead of using this packet only. The client may also send this packet to the server when it updates one of these settings through the in-game settings interface. The server should verify if the player actually has permission to update those settings.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `flags` | `uint32` | Flags is a set of flags that specify certain properties of the player, such as whether or not it can fly and/or move through blocks. It is one of the AdventureFlag constants above. |
| `command_permission_level` | `uint32` | CommandPermissionLevel is a permission level that specifies the kind of commands that the player is allowed to use. It is one of the CommandPermissionLevel constants above. |
| `action_permissions` | `uint32` | ActionPermissions is, much like Flags, a set of flags that specify actions that the player is allowed to undertake, such as whether it is allowed to edit blocks, open doors etc. It is a combination of the ActionPermission constants above. |
| `permission_level` | `uint32` | PermissionLevel is the permission level of the player as it shows up in the player list built up using the PlayerList packet. It is one of the PermissionLevel constants above. |
| `custom_stored_permissions` | `uint32` | CustomStoredPermissions ... |
| `player_unique_id` | `int64` | PlayerUniqueID is a unique identifier of the player. This must be filled out with the entity unique ID of the player. |

## 56 · BlockActorData · 方块实体数据

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：BlockActorData is sent by the server to update data of a block entity client-side, for example the data of a chest.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `position` | `protocol.BlockPos` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position of the block that holds the block entity. If no block entity is at this position, the packet is ignored by the client. |
| `nbtdata` | `map[string]interface {}` | NBTData is the new data of the block that will be encoded to NBT and applied client-side, so that the client can see the block update. The NBTData should contain all properties of the block, not just properties that were changed. |

## 58 · LevelChunk · 整块区块数据

- 分组：世界与结构
- 方向：服务器→客户端
- 用法与边界：完整区块解析涉及压缩、调色板与子区块。当前world.blockState仅最近UpdateBlock观测，不宣称完整世界缓存或寻路已做完。
- 本内核原始说明：LevelChunk is sent by the server to provide the client with a chunk of a world data (16xYx16 blocks). Typically, a certain amount of chunks is sent to the client before sending it the spawn PlayStatus packet, so that the client spawns in a loaded world.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `position` | `protocol.ChunkPos` | 位置坐标，维度与有效性按本包上下文确定。 Position contains the X and Z coordinates of the chunk sent. You can convert a block coordinate to a chunk coordinate by right-shifting it four bits. |
| `dimension` | `int32` | Dimension is the ID of the dimension that the chunk belongs to. This must always be set otherwise the client will always assume the chunk is part of the overworld dimension. |
| `highest_sub_chunk` | `uint16` | HighestSubChunk is the highest sub-chunk at the position that is not all air. It is only set if the SubChunkCount is set to protocol.SubChunkRequestModeLimited. |
| `sub_chunk_count` | `uint32` | SubChunkCount is the amount of sub-chunks that are part of the chunk sent. Depending on if the cache is enabled, a list of blob hashes will be sent, or, if disabled, the sub-chunk data. SubChunkCount may be set to protocol.SubChunkRequestModeLimited or protocol.SubChunkRequestModeLimitless to prompt the client to send a SubChunkRequest in response. If this field is set to protocol.SubChunkRequestModeLimited, HighestSubChunk is used. |
| `cache_enabled` | `bool` | CacheEnabled specifies if the client blob cache should be enabled. This system is based on hashes of blobs which are consistent and saved by the client in combination with that blob, so that the server does not have the same chunk multiple times. If the client does not yet have a blob with the hash sent, it will send a ClientCacheBlobStatus packet containing the hashes is does not have the data of. |
| `blob_hashes` | `[]uint64` | BlobHashes is a list of all blob hashes used in the chunk. It is composed of SubChunkCount + 1 hashes, with the first SubChunkCount hashes being those of the sub-chunks and the last one that of the biome of the chunk. If CacheEnabled is set to false, BlobHashes can be left empty. |
| `raw_payload` | `[]uint8` | RawPayload is a serialised string of chunk data. The data held depends on if CacheEnabled is set to true. If set to false, the payload is composed of multiple sub-chunks, each of which carry a version which indicates the way they are serialised, followed by biomes, border blocks and tile entities. If CacheEnabled is true, the payload consists out of the border blocks and tile entities only. |

## 59 · SetCommandsEnabled · 命令启用状态

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：服务器通知当前会话是否允许命令。
- 本内核原始说明：SetCommandsEnabled is sent by the server to enable or disable the ability to execute commands for the client. If disabled, the client itself will stop the execution of commands.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `enabled` | `bool` | Enabled defines if the commands should be enabled, or if false, disabled. |

## 60 · SetDifficulty · 世界难度

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：服务端同步难度设置，用于状态显示。
- 本内核原始说明：SetDifficulty is sent by the server to update the client-side difficulty of the client. The actual effect of this packet on the client isn't very significant, as the difficulty is handled server-side.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `difficulty` | `uint32` | Difficulty is the new difficulty that the world has. |

## 61 · ChangeDimension · 切换维度

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：维度与传送位置改变，应清除旧维度位置缓存。
- 本内核原始说明：ChangeDimension is sent by the server to the client to send a dimension change screen client-side. Once the screen is cleared client-side, the client will send a PlayerAction packet with PlayerActionDimensionChangeDone.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `dimension` | `int32` | Dimension is the dimension that the client should be changed to. The fog colour will change depending on the type of dimension, which is one of the constants above. Note that Dimension MUST be a different dimension than the one that the player is currently in. Sending a ChangeDimension packet with a Dimension that the player is currently in will result in a never-ending dimension change screen. |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position in the new dimension that the player is spawned in. |
| `respawn` | `bool` | Respawn specifies if the dimension change was respawn based, meaning that the player died in one dimension and got respawned into another. The client will send a PlayerAction packet with PlayerActionDimensionChangeRequest if it dies in another dimension, indicating that it needs a DimensionChange packet with Respawn set to true. |
| `loading_screen_id` | `protocol.Optional[uint32]` | LoadingScreenID is a unique ID for the loading screen that the player is currently in. The client will update the server on its state through the ServerBoundLoadingScreen packet, and it can be used to not send specific packets to the client if it is changing dimensions. This field should be unique for every ChangeDimension packet sent. |

## 62 · SetPlayerGameType · 玩家游戏模式设置

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：同步游戏模式，不能假定所有玩家模式相同。
- 本内核原始说明：SetPlayerGameType is sent by the server to update the game type, which is otherwise known as the game mode, of a player.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `game_type` | `int32` | GameType is the new game type of the player. It is one of the constants that can be found above. Some of these game types require additional flags to be set in an AdventureSettings packet for the game mode to obtain its full functionality. |

## 63 · PlayerList · 玩家名单变更

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：玩家加入/移除列表、UUID和皮肤资料；在线名单与皮肤缓存入口。
- 本内核原始说明：PlayerList is sent by the server to update the client-side player list in the in-game menu screen. It shows the icon of each player if the correct XUID is written in the packet. Sending the PlayerList packet is obligatory when sending an AddPlayer packet. The added player will not show up to a client if it has not been added to the player list, because several properties of the player are obtained from the player list, such as the skin.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `action_type` | `uint8` | 动作枚举，具体编号使用本内核定义。 ActionType is the action to execute upon the player list. The entries that follow specify which entries are added or removed from the player list. |
| `entries` | `[]protocol.PlayerListEntry` | Entries is a list of all player list entries that should be added/removed from the player list, depending on the ActionType set. |
| `unknown1` | `[]protocol.NeteaseUnknownPlayerListEntry` | Netease |
| `unknown2` | `[]protocol.NeteaseUnknownPlayerListEntry` | Netease |
| `unknown3` | `[]string` | Netease |
| `growth_levels` | `[]uint32` | Netease: launcher levels (1 - 50), such as "Diamond V", "Bedrock III".. |
| `unknown4` | `[]bool` | Netease |

### entries 嵌套字段

- `uuid` (`uuid.UUID`)：UUID is the UUID of the player as sent in the Login packet when the client joined the server. It must match this UUID exactly for the correct XBOX Live icon to show up in the list.
- `entity_unique_id` (`int64`)：EntityUniqueID is the unique entity ID of the player. This ID typically stays consistent during the lifetime of a world, but servers often send the runtime ID for this.
- `username` (`string`)：Username is the username that is shown in the player list of the player that obtains a PlayerList packet with this entry. It does not have to be the same as the actual username of the player.
- `xuid` (`string`)：XUID is the XBOX Live user ID of the player, which will remain consistent as long as the player is logged in with the XBOX Live account.
- `platform_chat_id` (`string`)：PlatformChatID is an identifier only set for particular platforms when chatting (presumably only for Nintendo Switch). It is otherwise an empty string, and is used to decide which players are able to chat with each other.
- `build_platform` (`int32`)：BuildPlatform is the platform of the player as sent by that player in the Login packet.
- `skin` (`Skin`)：Skin is the skin of the player that should be added to the player list. Once sent here, it will not have to be sent again.
- `skin.skin_id` (`string`)：SkinID is a unique ID produced for the skin, for example 'c18e65aa-7b21-4637-9b63-8ad63622ef01_Alex' for the default Alex skin.
- `skin.play_fab_id` (`string`)：PlayFabID is the PlayFab ID produced for the skin. PlayFab is the company that hosts the Marketplace, skins and other related features from the game. This ID is the ID of the skin used to store the skin inside of PlayFab.
- `skin.skin_resource_patch` (`[]byte`)：SkinResourcePatch is a JSON encoded object holding some fields that point to the geometry that the skin has. The JSON object that this holds specifies the way that the geometry of animations and the default skin of the player are combined.
- `skin.skin_data` (`[]byte`)：SkinData is a byte slice of SkinImageWidth * SkinImageHeight bytes. It is an RGBA ordered byte representation of the skin pixels.
- `skin.animations` (`[]SkinAnimation`)：Animations is a list of all animations that the skin has.
- `skin.animations.image_data` (`[]byte`)：ImageData is a byte slice of ImageWidth * ImageHeight bytes. It is an RGBA ordered byte representation of the animation image pixels. The ImageData contains FrameCount images in it, which each represent one stage of the animation. The actual part of the skin that this field holds depends on the AnimationType, where SkinAnimationHead holds only the head and its hat, whereas the other animations hold the entire body of the skin.
- `skin.animations.animation_type` (`uint32`)：AnimationType is the type of the animation, which is one of the types found above. The data that ImageData contains depends on this type.
- `skin.animations.frame_count` (`float32`)：FrameCount is the amount of frames that the skin animation holds. The number of frames here is the amount of images that may be found in the ImageData field.
- `skin.animations.expression_type` (`uint32`)：ExpressionType is the type of expression made by the skin, which is one the types found above.
- `skin.cape_data` (`[]byte`)：CapeData is a byte slice of 64*32*4 bytes. It is a RGBA ordered byte representation of the cape colours, much like the SkinData.
- `skin.skin_geometry` (`[]byte`)：SkinGeometry is a JSON encoded structure of the geometry data of a skin, containing properties such as bones, uv, pivot etc.
- `skin.animation_data` (`[]byte`)：TODO: Find out what value AnimationData holds and when it does hold something.
- `skin.geometry_data_engine_version` (`[]byte`)：GeometryDataEngineVersion ...
- `skin.premium_skin` (`bool`)：PremiumSkin specifies if this is a skin that was purchased from the marketplace.
- `skin.persona_skin` (`bool`)：PersonaSkin specifies if this is a skin that was created using the in-game skin creator.
- `skin.persona_cape_on_classic_skin` (`bool`)：PersonaCapeOnClassicSkin specifies if the skin had a Persona cape (in-game skin creator cape) equipped on a classic skin.
- `skin.primary_user` (`bool`)：PrimaryUser ...
- `skin.cape_id` (`string`)：CapeID is a unique identifier that identifies the cape. It usually holds a UUID in it.
- `skin.full_id` (`string`)：FullID is an ID that represents the skin in full. The actual functionality is unknown: The client does not seem to send a value for this.
- `skin.skin_colour` (`string`)：SkinColour is a hex representation (including #) of the base colour of the skin. An example of the colour sent here is '#b37b62'.
- `skin.arm_size` (`string`)：ArmSize is the size of the arms of the player's model. This is either 'wide' (generally for male skins) or 'slim' (generally for female skins).
- `skin.persona_pieces` (`[]PersonaPiece`)：PersonaPieces is a list of all persona pieces that the skin is composed of.
- `skin.persona_pieces.piece_id` (`string`)：PieceId is a UUID that identifies the piece itself, which is unique for each separate piece.
- `skin.persona_pieces.piece_type` (`string`)：PieceType holds the type of the piece. Several types I was able to find immediately are listed below. - persona_skeleton - persona_body - persona_skin - persona_bottom - persona_feet - persona_top - persona_mouth - persona_hair - persona_eyes - persona_facial_hair
- `skin.persona_pieces.pack_id` (`string`)：PackID is a UUID that identifies the pack that the persona piece belongs to.
- `skin.persona_pieces.default` (`bool`)：Default specifies if the piece is one of the default pieces. This is true when the piece is one of those that a Steve or Alex skin have.
- `skin.persona_pieces.product_id` (`string`)：ProductID is a UUID that identifies the piece when it comes to purchases. It is empty for pieces that have the 'Default' field set to true.
- `skin.piece_tint_colours` (`[]PersonaPieceTintColour`)：PieceTintColours is a list of specific tint colours for (some of) the persona pieces found in the list above.
- `skin.piece_tint_colours.piece_type` (`string`)：PieceType is the type of the persona skin piece that this tint colour concerns. The piece type must always be present in the persona pieces list, but not each piece type has a tint colour sent. Pieces that do have a tint colour that I was able to find immediately are listed below. - persona_mouth - persona_eyes - persona_hair
- `skin.piece_tint_colours.colours` (`[]string`)：Colours is a list four colours written in hex notation (note, that unlike the SkinColour field in the ClientData struct, this is actually ARGB, not just RGB). The colours refer to different parts of the skin piece. The 'persona_eyes' may have the following colours: ["#ffa12722","#ff2f1f0f","#ff3aafd9","#0"] The first hex colour represents the tint colour of the iris, the second hex colour represents the eyebrows and the third represents the sclera. The fourth is #0 because there are only 3 parts of the persona_eyes skin piece.
- `skin.trusted` (`bool`)：Trusted specifies if the skin is 'trusted'. No code should rely on this field, as any proxy or client can easily change it.
- `skin.override_appearance` (`bool`)：OverrideAppearance specifies if the skin should override the player's skin that is equipped client-side. When false, the client will reject the skin and continue to use the skin that the player has equipped.
- `teacher` (`bool`)：Teacher is a Minecraft: Education Edition field. It specifies if the player to be added to the player list is a teacher.
- `host` (`bool`)：Host specifies if the player that is added to the player list is the host of the game.
- `sub_client` (`bool`)：SubClient specifies if the player that is added to the player list is a sub-client of another player.
- `player_colour` (`color.RGBA`)：PlayerColour is the colour of the player that is shown in UI elements, currently only used for the player locator bar.

### unknown1 嵌套字段

- `has_value` (`bool`)：Netease: whether the entry has a value or not.
- `unknown1` (`bool`)：Netease
- `unknown2` (`string`)：Netease
- `unknown3` (`string`)：Netease
- `unknown4` (`string`)：Netease
- `unknown5` (`string`)：Netease
- `unknown6` (`string`)：Netease

### unknown2 嵌套字段

- `has_value` (`bool`)：Netease: whether the entry has a value or not.
- `unknown1` (`bool`)：Netease
- `unknown2` (`string`)：Netease
- `unknown3` (`string`)：Netease
- `unknown4` (`string`)：Netease
- `unknown5` (`string`)：Netease
- `unknown6` (`string`)：Netease

## 64 · SimpleEvent · 简单事件

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：由事件编号区分的少量状态事件，按当前内核枚举解释。
- 本内核原始说明：SimpleEvent is sent by the server to send a 'simple event' to the client, meaning an event without any additional event data. The event is typically used by the client for telemetry.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `event_type` | `int16` | EventType is the type of the event to be called. It is one of the constants that may be found above. |

## 65 · Event · 事件

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：Event is sent by the server to send an event with additional data. It is typically sent to the client for telemetry reasons, much like the SimpleEvent packet.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `int64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the player. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `use_player_id` | `uint8` | UsePlayerID ... TODO: Figure out what UsePlayerID is for. |
| `event` | `protocol.Event` | Event is the event that is transmitted. |

### event 嵌套字段

- `entity_runtime_id` (`int64`)：EntityRuntimeID is the runtime ID of the player. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID.
- `use_player_id` (`byte`)：UsePlayerID ... TODO: Figure out what UsePlayerID is for.
- `event` (`protocol.Event`)：Event is the event that is transmitted.
- `event.entity_runtime_id` (`int64`)：EntityRuntimeID is the runtime ID of the player. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID.
- `event.use_player_id` (`byte`)：UsePlayerID ... TODO: Figure out what UsePlayerID is for.
- `event.event` (`protocol.Event`)：Event is the event that is transmitted.
- `event.event.entity_runtime_id` (`int64`)：EntityRuntimeID is the runtime ID of the player. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID.
- `event.event.use_player_id` (`byte`)：UsePlayerID ... TODO: Figure out what UsePlayerID is for.
- `event.event.event` (`protocol.Event`)：Event is the event that is transmitted.
- `event.event.event.entity_runtime_id` (`int64`)：EntityRuntimeID is the runtime ID of the player. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID.
- `event.event.event.use_player_id` (`byte`)：UsePlayerID ... TODO: Figure out what UsePlayerID is for.
- `event.event.event.event` (`protocol.Event`)：Event is the event that is transmitted.

## 66 · SpawnExperienceOrb · 出生经验球

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：SpawnExperienceOrb is sent by the server to spawn an experience orb entity client-side. Much like the AddPainting packet, it is one of the few packets that spawn an entity without using the AddActor packet.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position to spawn the experience orb on. If the entity is on a distance that the player cannot see it, the entity will still show up if the player moves closer. |
| `experience_amount` | `int32` | ExperienceAmount is the amount of experience in experience points that the orb carries. The client-side size of the orb depends on the amount of experience in the orb: There are 11 possible sizes for the orb, for 1–2, 3–6, 7–16, 17–36, 37–72, 73–148, 149–306, 307–616, 617–1236, 1237–2476, and 2477 and up. |

## 67 · ClientBoundMapItemData · 客户端定向地图物品数据

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ClientBoundMapItemData is sent by the server to the client to update the data of a map shown to the client. It is sent with a combination of flags that specify what data is updated. The ClientBoundMapItemData packet may be used to update specific parts of the map only. It is not required to send the entire map each time when updating one part.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `map_id` | `int64` | MapID is the unique identifier that represents the map that is updated over network. It remains consistent across sessions. |
| `update_flags` | `uint32` | UpdateFlags is a combination of flags found above that indicate what parts of the map should be updated client-side. |
| `dimension` | `uint8` | Dimension is the dimension of the map that should be updated, for example the overworld (0), the nether (1) or the end (2). |
| `locked_map` | `bool` | LockedMap specifies if the map that was updated was a locked map, which may be done using a cartography table. |
| `origin` | `protocol.BlockPos` | Origin is the center position of the map being updated. |
| `scale` | `uint8` | Scale is the scale of the map as it is shown in-game. It is written when any of the MapUpdateFlags are set to the UpdateFlags field. |
| `maps_included_in` | `[]int64` | The following fields apply only for the MapUpdateFlagInitialisation. MapsIncludedIn holds an array of map IDs that the map updated is included in. This has to do with the scale of the map: Each map holds its own map ID and all map IDs of maps that include this map and have a bigger scale. This means that a scale 0 map will have 5 map IDs in this slice, whereas a scale 4 map will have only 1 (its own). The actual use of this field remains unknown. |
| `tracked_objects` | `[]protocol.MapTrackedObject` | The following fields apply only for the MapUpdateFlagDecoration. TrackedObjects is a list of tracked objects on the map, which may either be entities or blocks. The client makes sure these tracked objects are actually tracked. (position updated etc.) |
| `decorations` | `[]protocol.MapDecoration` | Decorations is a list of fixed decorations located on the map. The decorations will not change client-side, unless the server updates them. |
| `height` | `int32` | The following fields apply only for the MapUpdateFlagTexture update flag. Height is the height of the texture area that was updated. The height may be a subset of the total height of the map. |
| `width` | `int32` | Width is the width of the texture area that was updated. The width may be a subset of the total width of the map. |
| `xoffset` | `int32` | XOffset is the X offset in pixels at which the updated texture area starts. From this X, the updated texture will extend exactly Width pixels to the right. |
| `yoffset` | `int32` | YOffset is the Y offset in pixels at which the updated texture area starts. From this Y, the updated texture will extend exactly Height pixels up. |
| `pixels` | `[]color.RGBA` | Pixels is a list of pixel colours for the new texture of the map. It is indexed as Pixels[y*height + x]. |

### tracked_objects 嵌套字段

- `type` (`int32`)：Type is the type of the tracked object. It is either MapObjectTypeEntity or MapObjectTypeBlock.
- `entity_unique_id` (`int64`)：EntityUniqueID is the unique ID of the entity, if the tracked object was an entity. It needs not to be filled out if Type is not MapObjectTypeEntity.
- `block_position` (`BlockPos`)：BlockPosition is the position of the block, if the tracked object was a block. It needs not to be filled out if Type is not MapObjectTypeBlock.

### decorations 嵌套字段

- `type` (`byte`)：Type is the type of the map decoration. The type specifies the shape (and sometimes the colour) that the map decoration gets. It is one of the MapDecorationType constants above.
- `rotation` (`byte`)：Rotation is the rotation of the map decoration. It is byte due to the 16 fixed directions that the map decoration may face.
- `x` (`byte`)：X is the offset on the X axis in pixels of the decoration.
- `y` (`byte`)：Y is the offset on the Y axis in pixels of the decoration.
- `label` (`string`)：Label is the name of the map decoration. This name may be of any value.
- `colour` (`color.RGBA`)：Colour is the colour of the map decoration. Some map decoration types have a specific colour set automatically, whereas others may be changed.

## 68 · MapInfoRequest · 地图信息请求

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：MapInfoRequest is sent by the client to request the server to deliver information of a certain map in the inventory of the player. The server should respond with a ClientBoundMapItemData packet.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `map_id` | `int64` | MapID is the unique identifier that represents the map that is requested over network. It remains consistent across sessions. |
| `client_pixels` | `[]protocol.PixelRequest` | ClientPixels is a slice of pixels sent from the client to notify the server about the pixels that it isn't aware of. |

### client_pixels 嵌套字段

- `colour` (`color.RGBA`)：
- `index` (`uint16`)：

## 69 · RequestChunkRadius · 本机请求视距

- 分组：世界与结构
- 方向：客户端→服务器
- 用法与边界：服务器可裁剪范围，应读ChunkRadiusUpdated确认，不能无条件强迫加载全图。
- 本内核原始说明：RequestChunkRadius is sent by the client to the server to update the server on the chunk view radius that it has set in the settings. The server may respond with a ChunkRadiusUpdated packet with either the chunk radius requested, or a different chunk radius if the server chooses so.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `chunk_radius` | `int32` | 区块半径，服务器可限制实际接受值。 ChunkRadius is the requested chunk radius. This value is always the value set in the settings of the player. |
| `max_chunk_radius` | `int32` | 协议允许的最大半径参数。 MaxChunkRadius is the maximum chunk radius that the player wants to receive. The reason for the client sending this is currently unknown. |

## 70 · ChunkRadiusUpdated · 区块视距响应

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：服务器实际接受的区块半径，配合RequestChunkRadius核对。
- 本内核原始说明：ChunkRadiusUpdated is sent by the server in response to a RequestChunkRadius packet. It defines the chunk radius that the server allows the client to have. This may be lower than the chunk radius requested by the client in the RequestChunkRadius packet.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `chunk_radius` | `int32` | 区块半径，服务器可限制实际接受值。 ChunkRadius is the final chunk radius that the client will adapt when it receives the packet. It does not have to be the same as the requested chunk radius. |

## 72 · GameRulesChanged · 游戏规则变更

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：服务端同步规则值，可查看收到的世界规则。
- 本内核原始说明：GameRulesChanged is sent by the server to the client to update client-side game rules, such as game rules like the 'showCoordinates' game rule.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `game_rules` | `[]protocol.GameRule` | GameRules defines game rules changed with their respective values. The value of these game rules may be either 'bool', 'int32' or 'float32'. Note that some game rules are server side only, and don't necessarily need to be sent to the client. Only changed game rules need to be sent in this packet. Game rules that were not changed do not need to be sent if the client is already updated on them. |

### game_rules 嵌套字段

- `name` (`string`)：Name is the name of the game rule.
- `can_be_modified_by_player` (`bool`)：CanBeModifiedByPlayer specifies if the game rule can be modified by the player through the in-game UI.
- `value` (`any`)：Value is the new value of the game rule. This is either a bool, uint32 or float32.

## 73 · Camera · 摄像机

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：Camera is sent by the server to use an Education Edition camera on a player. It produces an image client-side.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `camera_entity_unique_id` | `int64` | CameraEntityUniqueID is the unique ID of the camera entity from which the picture was taken. |
| `target_player_unique_id` | `int64` | TargetPlayerUniqueID is the unique ID of the target player. The unique ID is a value that remains consistent across different sessions of the same world, but most servers simply fill the runtime ID of the player out for this field. |

## 74 · BossEvent · 首领血条事件

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：首领栏显示、血条比例和标题；不是任意生物的真实血量查询。
- 本内核原始说明：BossEvent is sent by the server to make a specific 'boss event' occur in the world. It includes features such as showing a boss bar to the player and turning the sky dark.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `boss_entity_unique_id` | `int64` | BossEntityUniqueID is the unique ID of the boss entity that the boss event sent involves. By default, the health percentage and title of the boss bar depend on the health and name tag of this entity. If BossEntityUniqueID is the same as the client's entity unique ID, its HealthPercentage and BossBarTitle can be freely altered. |
| `event_type` | `uint32` | EventType is the type of the event. The fields written depend on the event type set, and some event types are sent by the client, whereas others are sent by the server. The event type is one of the constants above. |
| `player_unique_id` | `int64` | PlayerUniqueID is the unique ID of the player that is registered to or unregistered from the boss fight. It is set if EventType is either BossEventRegisterPlayer or BossEventUnregisterPlayer. |
| `boss_bar_title` | `string` | BossBarTitle is the title shown above the boss bar. It may be set to set a different title if the BossEntityUniqueID matches the client's entity unique ID. |
| `filtered_boss_bar_title` | `string` | FilteredBossBarTitle is a filtered version of BossBarTitle with all the profanity removed. The client will use this over BossBarTitle if this field is not empty and they have the "Filter Profanity" setting enabled. |
| `health_percentage` | `float32` | HealthPercentage is the percentage of health that is shown in the boss bar (0.0-1.0). The HealthPercentage may be set to a specific value if the BossEntityUniqueID matches the client's entity unique ID. |
| `screen_darkening` | `uint16` | ScreenDarkening currently seems not to do anything. |
| `colour` | `uint32` | Colour is the colour of the boss bar that is shown when a player is subscribed. It is only set if the EventType is BossEventShow, BossEventAppearanceProperties or BossEventTexture. This is functional as of 1.18 and can be any of the BossEventColour constants listed above. |
| `overlay` | `uint32` | Overlay is the overlay of the boss bar that is shown on top of the boss bar when a player is subscribed. It currently does not function. It is only set if the EventType is BossEventShow, BossEventAppearanceProperties or BossEventTexture. |

## 75 · ShowCredits · 结束鸣谢画面

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：游戏结束鸣谢界面状态，通常由服务器控制。
- 本内核原始说明：ShowCredits is sent by the server to show the Minecraft credits screen to the client. It is typically sent when the player beats the ender dragon and leaves the End.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `player_runtime_id` | `uint64` | PlayerRuntimeID is the entity runtime ID of the player to show the credits to. It's not clear why this field is actually here in the first place. |
| `status_type` | `int32` | StatusType is the status type of the credits. It is one of the constants above, and either starts or stops the credits. |

## 76 · AvailableCommands · 服务器可用命令与参数

- 分组：命令与计分板
- 方向：服务器→客户端
- 用法与边界：命令名、描述、权限、重载、枚举等；适合AI查询真实服务器支持的命令，不代表可提升权限。
- 本内核原始说明：AvailableCommands is sent by the server to send a list of all commands that the player is able to use on the server. This packet holds all the arguments of each commands as well, making it possible for the client to provide auto-completion and command usages. AvailableCommands packets can be resent, but the packet is often very big, so doing this very often should be avoided.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `enum_values` | `[]string` | EnumValues is a slice of all enum values of any enum in the AvailableCommands packet. EnumValues generally should contain each possible value only once. Enums are built by pointing to entries in this slice. |
| `chained_subcommand_values` | `[]string` | ChainedSubcommandValues is a slice of all chained subcommand names. ChainedSubcommandValues generally should contain each possible value only once. ChainedSubcommands are built by pointing to entries in this slice. |
| `suffixes` | `[]string` | Suffixes, like EnumValues, is a slice of all suffix values of any command parameter in the AvailableCommands packet. |
| `enums` | `[]protocol.CommandEnum` | Enums is a slice of all (fixed) command enums present in any of the commands. |
| `chained_subcommands` | `[]protocol.ChainedSubcommand` | ChainedSubcommands is a slice of all subcommands that are followed by a chained command. An example usage of this is /execute which allows you to run another command as another entity or at a different position etc. |
| `commands` | `[]protocol.Command` | Commands is a list of all commands that the client should show client-side. The AvailableCommands packet replaces any commands sent before. It does not only add the commands that are sent in it. |
| `dynamic_enums` | `[]protocol.DynamicEnum` | DynamicEnums is a slice of dynamic command enums. These command enums can be changed during runtime without having to resend an AvailableCommands packet. |
| `constraints` | `[]protocol.CommandEnumConstraint` | Constraints is a list of constraints that should be applied to certain options of enums in the commands above. |

### enums 嵌套字段

- `type` (`string`)：Type is the type of the command enum. The type will show up in the command usage as the type of the argument if it has a certain amount of arguments, or when Options is set to true in the command holding the enum.
- `value_indices` (`[]uint`)：ValueIndices holds a list of indices that point to the EnumValues slice in the AvailableCommandsPacket. These represent the options of the enum.

### chained_subcommands 嵌套字段

- `name` (`string`)：Name is the name of the chained subcommand and shows up in the list as a regular subcommand enum.
- `values` (`[]ChainedSubcommandValue`)：Values contains the index and parameter type of the chained subcommand.
- `values.index` (`uint16`)：Index is the index of the argument in the ChainedSubcommandValues slice from the AvailableCommands packet. This is then used to set the type specified by the Value field below.
- `values.value` (`uint16`)：Value is a combination of the flags above and specified the type of argument. Unlike regular parameter types, this should NOT contain any of the special flags (valid, enum, suffixed or soft enum) but only the basic types.

### commands 嵌套字段

- `name` (`string`)：Name is the name of the command. The command may be executed using this name, and will be shown in the /help list with it. It currently seems that the client crashes if the Name contains uppercase letters.
- `description` (`string`)：Description is the description of the command. It is shown in the /help list and when starting to write a command.
- `flags` (`uint16`)：Flags is a combination of flags not currently known. Leaving the Flags field empty appears to work.
- `permission_level` (`byte`)：PermissionLevel is the command permission level that the player required to execute this command. The field no longer seems to serve a purpose, as the client does not handle the execution of commands anymore: The permissions should be checked server-side.
- `aliases_offset` (`uint32`)：AliasesOffset is the offset to a CommandEnum that holds the values that should be used as aliases for this command.
- `chained_subcommand_offsets` (`[]uint16`)：ChainedSubcommandOffsets is a slice of offsets that all point to a different ChainedSubcommand from the ChainedSubcommands slice in the AvailableCommands packet.
- `overloads` (`[]CommandOverload`)：Overloads is a list of command overloads that specify the ways in which a command may be executed. The overloads may be completely different.
- `overloads.chaining` (`bool`)：Chaining determines if the parameters use chained subcommands or not.
- `overloads.parameters` (`[]CommandParameter`)：Parameters is a list of command parameters that are part of the overload. These parameters specify the usage of the command when this overload is applied.
- `overloads.parameters.name` (`string`)：Name is the name of the command parameter. It shows up in the usage like <$Name: $Type>, with the exception of enum types, which show up simply as a list of options if the list is short enough and Options is set to false.
- `overloads.parameters.type` (`uint32`)：Type is a rather odd combination of type(flag)s that result in a certain parameter type to show up client-side. It is a combination of the flags above. The basic types must be combined with the ArgumentTypeFlagBasic flag (and integers with a suffix ArgumentTypeFlagSuffixed), whereas enums are combined with the ArgumentTypeFlagEnum flag.
- `overloads.parameters.optional` (`bool`)：Optional specifies if the command parameter is optional to enter. Note that no non-optional parameter should ever be present in a command overload after an optional parameter. When optional, the parameter shows up like so: [$Name: $Type], whereas when mandatory, it shows up like so: <$Name: $Type>.
- `overloads.parameters.options` (`byte`)：Options holds a combinations of options that additionally apply to the command parameter. The list of options can be found above.

### dynamic_enums 嵌套字段

- `type` (`string`)：Type is the type of the command enum. The type will show up in the command usage as the type of the argument if it has a certain amount of arguments, or when Options is set to true in the command holding the enum.
- `values` (`[]string`)：Values is a slice of possible options for the enum.

### constraints 嵌套字段

- `enum_value_index` (`uint32`)：EnumValueIndex points to an enum value in the AvailableCommands packet that this constraint should apply to.
- `enum_index` (`uint32`)：EnumIndex points to an enum in the AvailableCommands packet to which this constraint should apply to.
- `constraints` (`[]byte`)：Constraints holds a slice of constraints as present in the constants above.

## 77 · CommandRequest · 命令执行请求

- 分组：命令与计分板
- 方向：客户端→服务器
- 用法与边界：携带CommandLine和CommandOrigin；确认路径匹配UUID返回CommandOutput。需要实际权限。
- 本内核原始说明：CommandRequest is sent by the client to request the execution of a server-side command. Although some servers support sending commands using the Text packet, this packet is guaranteed to have the correct result.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `command_line` | `string` | CommandLine is the raw entered command line. The client does no parsing of the command line by itself (unlike it did in the early stages), but lets the server do that. |
| `command_origin` | `protocol.CommandOrigin` | CommandOrigin is the data specifying the origin of the command. In other words, the source that the command was from, such as the player itself or a websocket server. |
| `internal` | `bool` | Internal specifies if the command request internal. Setting it to false seems to work and the usage of this field is not known. |
| `version` | `int32` | Version is the version of the command that is being executed. This field currently has no purpose or functionality. |
| `un_limited` | `bool` | PhoenixBuilder specific fields. Author: LNSSPsd  NetEase specified field |

### command_origin 嵌套字段

- `origin` (`uint32`)：Origin is one of the values above that specifies the origin of the command. The origin may change, depending on what part of the client actually called the command. The command may be issued by a websocket server, for example.
- `uuid` (`uuid.UUID`)：UUID is a unique identifier for every instantiation of a command.
- `request_id` (`string`)：RequestID is an ID that identifies the request of the client. The server should send a CommandOrigin with the same request ID to ensure it can be matched with the request by the caller of the command. This is especially important for websocket servers and it seems that this field is only non-empty for these websocket servers.
- `player_unique_id` (`int64`)：PlayerUniqueID is an ID that identifies the player, the same as the one found in the AdventureSettings packet. Filling it out with 0 seems to work. PlayerUniqueID is only written if Origin is CommandOriginDevConsole or CommandOriginTest.

## 78 · CommandBlockUpdate · 命令方块更新

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：CommandBlockUpdate is sent by the client to update a command block at a specific position. The command block may be either a physical block or an entity.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `block` | `bool` | Block specifies if the command block updated was an actual physical block. If false, the command block is in a minecart and has an entity runtime ID instead. |
| `position` | `protocol.BlockPos` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position of the command block updated. It is only set if Block is set to true. Nothing happens if no command block is set at this position. |
| `mode` | `uint32` | Mode is the mode of the command block. It is either CommandBlockImpulse, CommandBlockChain or CommandBlockRepeat. It is only set if Block is set to true. |
| `needs_redstone` | `bool` | NeedsRedstone specifies if the command block needs to be powered by redstone to be activated. If false, the command block is always active. The field is only set if Block is set to true. |
| `conditional` | `bool` | Conditional specifies the behaviour of the command block if the command block before it (the opposite side of the direction the arrow if facing) fails to execute. If set to false, it will activate at all times, whereas if set to true, it will activate only if the previous command block executed successfully. The field is only set if Block is set to true. |
| `minecart_entity_runtime_id` | `uint64` | MinecartEntityRuntimeID is the runtime ID of the minecart entity carrying the command block that is updated. It is set only if Block is set to false. |
| `command` | `string` | Command is the command currently entered in the command block. This is the command that is executed when the command block is activated. |
| `last_output` | `string` | LastOutput is the output of the last command executed by the command block. It may be left empty to show simply no output at all, in combination with setting ShouldTrackOutput to false. |
| `name` | `string` | Name is the name of the command block updated. If not empty, it will show this name hovering above the command block when hovering over the block with the cursor. |
| `filtered_name` | `string` | FilteredName is a filtered version of Name with all the profanity removed. The client will use this over Name if this field is not empty and they have the "Filter Profanity" setting enabled. |
| `should_track_output` | `bool` | ShouldTrackOutput specifies if the command block tracks output. If set to false, the output box won't be shown within the command block. |
| `execute_on_first_tick` | `bool` | ExecuteOnFirstTick specifies if the command block should execute on the first tick, AKA as soon as the command block is enabled. |
| `tick_delay` | `int32` | TickDelay is the delay in ticks between executions of a command block, if it is a repeating command block. |

## 79 · CommandOutput · 命令执行结果

- 分组：命令与计分板
- 方向：服务器→客户端
- 用法与边界：SuccessCount/消息参数/DataSet，判断成功与条件分支；超时仍是未知，不能当失败后自动重发副作用。
- 本内核原始说明：CommandOutput is sent by the server to the client to send text as output of a command. Most servers do not use this packet and instead simply send Text packets, but there is reason to send it. If the origin of a CommandRequest packet is not the player itself, but, for example, a websocket server, sending a Text packet will not do what is expected: The message should go to the websocket server, not to the client's chat. The CommandOutput packet will make sure the messages are relayed to the correct origin of the command request.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `command_origin` | `protocol.CommandOrigin` | CommandOrigin is the data specifying the origin of the command. In other words, the source that the command request was from, such as the player itself or a websocket server. The client forwards the messages in this packet to the right origin, depending on what is sent here. |
| `output_type` | `uint8` | OutputType specifies the type of output that is sent. The OutputType sent by vanilla games appears to be 3, which seems to work. |
| `success_count` | `uint32` | 服务端报告成功的数量。 SuccessCount is the amount of times that a command was executed successfully as a result of the command that was requested. For servers, this is usually a rather meaningless fields, but for vanilla, this is applicable for commands created with Functions. |
| `output_messages` | `[]protocol.CommandOutputMessage` | OutputMessages is a list of all output messages that should be sent to the player. Whether they are shown or not, depends on the type of the messages. |
| `data_set` | `string` | DataSet ... TODO: Find out what this is for. |

### command_origin 嵌套字段

- `origin` (`uint32`)：Origin is one of the values above that specifies the origin of the command. The origin may change, depending on what part of the client actually called the command. The command may be issued by a websocket server, for example.
- `uuid` (`uuid.UUID`)：UUID is a unique identifier for every instantiation of a command.
- `request_id` (`string`)：RequestID is an ID that identifies the request of the client. The server should send a CommandOrigin with the same request ID to ensure it can be matched with the request by the caller of the command. This is especially important for websocket servers and it seems that this field is only non-empty for these websocket servers.
- `player_unique_id` (`int64`)：PlayerUniqueID is an ID that identifies the player, the same as the one found in the AdventureSettings packet. Filling it out with 0 seems to work. PlayerUniqueID is only written if Origin is CommandOriginDevConsole or CommandOriginTest.

### output_messages 嵌套字段

- `success` (`bool`)：Success indicates if the output message was one of a successful command execution. If set to true, the output message is by default coloured white, whereas if set to false, the message is by default coloured red.
- `message` (`string`)：Message is the message that is sent to the client in the chat window. It may either be simply a message or a translated built-in string like 'commands.tp.success.coordinates', combined with specific parameters below.
- `parameters` (`[]string`)：Parameters is a list of parameters that serve to supply the message sent with additional information, such as the position that a player was teleported to or the effect that was applied to an entity. These parameters only apply for the Minecraft built-in command output.

## 80 · UpdateTrade · 本机村民交易内容

- 分组：物品与容器
- 方向：服务器→本机客户端
- 用法与边界：所打开交易界面的配方信息；不能伪造别人交易价格或改他人交易。
- 本内核原始说明：UpdateTrade is sent by the server to update the trades offered by a villager to a player. It is sent at the moment that a player interacts with a villager.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `window_id` | `uint8` | 当前容器窗口ID。 WindowID is the ID that identifies the trading window that the client currently has opened. |
| `window_type` | `uint8` | WindowType is an identifier specifying the type of the window opened. In vanilla, it appears this is always filled out with 15. |
| `size` | `int32` | Size is the amount of trading options that the villager has. |
| `trade_tier` | `int32` | TradeTier is the tier of the villager that the player is trading with. The tier starts at 0 with a first two offers being available, after which two additional offers are unlocked each time the tier becomes one higher. |
| `villager_unique_id` | `int64` | VillagerUniqueID is the unique ID of the villager entity that the player is trading with. The TradeTier sent above applies to this villager. |
| `entity_unique_id` | `int64` | 实体唯一ID，与运行ID不同。 EntityUniqueID is the unique ID of the entity (usually a player) for which the trades are updated. The updated trades may apply only to this entity. |
| `display_name` | `string` | DisplayName is the name displayed at the top of the trading UI. It is usually used to represent the profession of the villager in the UI. |
| `new_trade_ui` | `bool` | NewTradeUI specifies if the villager should be using the new trade UI (The one added in 1.11.) rather than the old one. This should usually be set to true. |
| `demand_based_prices` | `bool` | DemandBasedPrices specifies if the prices of the villager's offers are modified by an increase in demand for the item. (A mechanic added in 1.11.) Buying more of the same item will increase the price of that particular item. |
| `serialised_offers` | `[]uint8` | SerialisedOffers is a network NBT serialised compound of offers that the villager has. |

## 81 · UpdateEquip · 更新装备

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：UpdateEquip is sent by the server to the client upon opening a horse inventory. It is used to set the content of the inventory and specify additional properties, such as the items that are allowed to be put in slots of the inventory.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `window_id` | `uint8` | 当前容器窗口ID。 WindowID is the identifier associated with the window that the UpdateEquip packet concerns. It is the ID sent for the horse inventory that was opened before this packet was sent. |
| `window_type` | `uint8` | WindowType is the type of the window that was opened. Generally, this is the type of a horse inventory, as the packet is specifically made for that. |
| `size` | `int32` | Size is the size of the horse inventory that should be opened. A bigger size does, in fact, change the amount of slots displayed. |
| `entity_unique_id` | `int64` | 实体唯一ID，与运行ID不同。 EntityUniqueID is the unique ID of the entity whose equipment was 'updated' to the player. It is typically the horse entity that had its inventory opened. |
| `serialised_inventory_data` | `[]uint8` | SerialisedInventoryData is a network NBT serialised compound holding the content of the inventory of the entity (the equipment) and additional data such as the allowed items for a particular slot, used to make sure only saddles can be put in the saddle slot etc. |

## 82 · ResourcePackDataInfo · 资源包数据信息

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ResourcePackDataInfo is sent by the server to the client to inform the client about the data contained in one of the resource packs that are about to be sent.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `uuid` | `string` | UUID is the unique ID of the resource pack that the info concerns. |
| `data_chunk_size` | `uint32` | DataChunkSize is the maximum size in bytes of the chunks in which the total size of the resource pack to be sent will be divided. A size of 1MB (1024*1024) means that a resource pack of 15.5MB will be split into 16 data chunks. |
| `chunk_count` | `uint32` | ChunkCount is the total amount of data chunks that the sent resource pack will exist out of. It is the total size of the resource pack divided by the DataChunkSize field. The client doesn't actually seem to use this field. Rather, it divides the size by the chunk size to calculate it itself. |
| `size` | `uint64` | Size is the total size in bytes that the resource pack occupies. This is the size of the compressed archive (zip) of the resource pack. |
| `hash` | `[]uint8` | Hash is a SHA256 hash of the content of the resource pack. |
| `premium` | `bool` | Premium specifies if the resource pack was a premium resource pack, meaning it was bought from the Minecraft store. |
| `pack_type` | `uint8` | PackType is the type of the resource pack. It is one of the resource pack types that may be found in the constants above. |

## 83 · ResourcePackChunkData · 资源包区块数据

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ResourcePackChunkData is sent to the client so that the client can download the resource pack. Each packet holds a chunk of the compressed resource pack, of which the size is defined in the ResourcePackDataInfo packet sent before.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `uuid` | `string` | UUID is the unique ID of the resource pack that the chunk of data is taken out of. |
| `chunk_index` | `uint32` | ChunkIndex is the current chunk index of the chunk. It is a number that starts at 0 and is incremented for each resource pack data chunk sent to the client. |
| `data_offset` | `uint64` | DataOffset is the current progress in bytes or offset in the data that the resource pack data chunk is taken from. |
| `data` | `[]uint8` | RawPayload is a byte slice containing a chunk of data from the resource pack. It must be of the same size or less than the DataChunkSize set in the ResourcePackDataInfo packet. |

## 84 · ResourcePackChunkRequest · 资源包区块请求

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ResourcePackChunkRequest is sent by the client to request a chunk of data from a particular resource pack, that it has obtained information about in a ResourcePackDataInfo packet.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `uuid` | `string` | UUID is the unique ID of the resource pack that the chunk of data is requested from. |
| `chunk_index` | `uint32` | ChunkIndex is the requested chunk index of the chunk. It is a number that starts at 0 and is incremented for each resource pack data chunk requested. |

## 85 · Transfer · 转移

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：Transfer is sent by the server to transfer a player from the current server to another. Doing so will fully disconnect the client, bring it back to the main menu and make it connect to the next server.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `address` | `string` | Address is the address of the new server, which might be either a hostname or an actual IP address. |
| `port` | `uint16` | Port is the UDP port of the new server. |
| `reload_world` | `bool` | ReloadWorld currently has an unknown usage. |

## 86 · PlaySound · 位置声音播放

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：声音名、坐标、音量与音高；须客户端存在对应声音资源。
- 本内核原始说明：PlaySound is sent by the server to play a sound to the client. Some of the sounds may only be started using this packet and must be stopped using the StopSound packet.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `sound_name` | `string` | SoundName is the name of the sound to play. |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position at which the sound was played. Some sounds do not depend on a position, which will then ignore it, but most of them will play with the direction based on the position compared to the player's position. |
| `volume` | `float32` | Volume is the relative volume of the sound to play. It will be less loud for the player if it is farther away from the position of the sound. |
| `pitch` | `float32` | 俯仰角。 Pitch is the pitch of the sound to play. Some sounds completely ignore this field, whereas others use it to specify the pitch as the field is intended. |

## 87 · StopSound · 停止声音

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：StopSound is sent by the server to stop a sound playing to the player, such as a playing music disk track or other long-lasting sounds.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `sound_name` | `string` | SoundName is the name of the sound that should be stopped from playing. If no sound with this name is currently active, the packet is ignored. |
| `stop_all` | `bool` | StopAll specifies if all sounds currently playing to the player should be stopped. If set to true, the SoundName field may be left empty. |
| `stop_music_legacy` | `bool` | StopMusicLegacy is currently unknown. |

## 88 · SetTitle · 标题与动作栏

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：标题、副标题、动作栏和显示时间的服务端通知。
- 本内核原始说明：SetTitle is sent by the server to make a title, subtitle or action bar shown to a player. It has several fields that allow setting the duration of the titles.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `action_type` | `int32` | 动作枚举，具体编号使用本内核定义。 ActionType is the type of the action that should be executed upon the title of a player. It is one of the constants above and specifies the response of the client to the packet. |
| `text` | `string` | Text is the text of the title, which has a different meaning depending on the ActionType that the packet has. The text is the text of a title, subtitle or action bar, depending on the type set. |
| `fade_in_duration` | `int32` | FadeInDuration is the duration that the title takes to fade in on the screen of the player. It is measured in 20ths of a second (AKA in ticks). |
| `remain_duration` | `int32` | RemainDuration is the duration that the title remains on the screen of the player. It is measured in 20ths of a second (AKA in ticks). |
| `fade_out_duration` | `int32` | FadeOutDuration is the duration that the title takes to fade out of the screen of the player. It is measured in 20ths of a second (AKA in ticks). |
| `xuid` | `string` | XUID is the XBOX Live user ID of the player, which will remain consistent as long as the player is logged in with the XBOX Live account. It is empty if the user is not logged into its XBL account. |
| `platform_online_id` | `string` | PlatformOnlineID is either a uint64 or an empty string. |
| `filtered_message` | `string` | FilteredMessage is a filtered version of Message with all the profanity removed. The client will use this over Message if this field is not empty and they have the "Filter Profanity" setting enabled. |

## 89 · AddBehaviourTree · 添加行为树

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：AddBehaviourTree is sent by the server to the client. The packet is currently unused by both client and server.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `behaviour_tree` | `string` | BehaviourTree is an unused string. |

## 90 · StructureBlockUpdate · 结构方块更新

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：StructureBlockUpdate is sent by the client when it updates a structure block using the in-game UI. The data it contains depends on the type of structure block that it is. In Minecraft Bedrock Edition v1.11, there is only the Export structure block type, but in v1.13 the ones present in Java Edition will, according to the wiki, be added too.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `position` | `protocol.BlockPos` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position of the structure block that is updated. |
| `structure_name` | `string` | StructureName is the name of the structure that was set in the structure block's UI. This is the name used to export the structure to a file. |
| `filtered_structure_name` | `string` | FilteredStructureName is a filtered version of StructureName with all the profanity removed. The client will use this over StructureName if this field is not empty and they have the "Filter Profanity" setting enabled. |
| `data_field` | `string` | DataField is the name of a function to run, usually used during natural generation. A description can be found here: https://minecraft.wiki/w/Structure_Block#Data. |
| `include_players` | `bool` | IncludePlayers specifies if the 'Include Players' toggle has been enabled, meaning players are also exported by the structure block. |
| `show_bounding_box` | `bool` | ShowBoundingBox specifies if the structure block should have its bounds outlined. A thin line will encapsulate the bounds of the structure if set to true. |
| `structure_block_type` | `int32` | StructureBlockType is the type of the structure block updated. A list of structure block types that will be used can be found in the constants above. |
| `settings` | `protocol.StructureSettings` | Settings is a struct of settings that should be used for exporting the structure. These settings are identical to the last sent in the StructureBlockUpdate packet by the client. |
| `redstone_save_mode` | `int32` | RedstoneSaveMode is the mode that should be used to save the structure when used with redstone. In Java Edition, this is always stored in memory, but in Bedrock Edition it can be stored either to disk or memory. See the constants above for the options. |
| `should_trigger` | `bool` | ShouldTrigger specifies if the structure block should be triggered immediately after this packet reaches the server. |
| `waterlogged` | `bool` | Waterlogged specifies if non-air blocks replace water or combine with water. |

### settings 嵌套字段

- `palette_name` (`string`)：PaletteName is the name of the palette used in the structure. Currently, it seems that this field is always 'default'.
- `ignore_entities` (`bool`)：IgnoreEntities specifies if the structure should ignore entities or include them. If set to false, entities will also show up in the exported structure.
- `ignore_blocks` (`bool`)：IgnoreBlocks specifies if the structure should ignore blocks or include them. If set to false, blocks will show up in the exported structure.
- `allow_non_ticking_chunks` (`bool`)：AllowNonTickingChunks specifies if the structure should allow non-ticking chunks. If set to false, the structure will export non-ticking chunks.
- `size` (`BlockPos`)：Size is the size of the area that is about to be exported. The area exported will start at the Position + Offset, and will extend as far as Size specifies.
- `offset` (`BlockPos`)：Offset is the offset position that was set in the structure block. The area exported is offset by this position.
- `last_editing_player_unique_id` (`int64`)：LastEditingPlayerUniqueID is the unique ID of the player that last edited the structure block that these settings concern.
- `rotation` (`byte`)：Rotation is the rotation that the structure block should obtain. See the constants above for available options.
- `mirror` (`byte`)：Mirror specifies the way the structure should be mirrored. It is either no mirror at all, mirror on the x/z axis or both.
- `animation_mode` (`byte`)：AnimationMode ...
- `animation_duration` (`float32`)：AnimationDuration ...
- `integrity` (`float32`)：Integrity is usually 1, but may be set to a number between 0 and 1 to omit blocks randomly, using the Seed that follows.
- `seed` (`uint32`)：Seed is the seed used to omit blocks if Integrity is not equal to one. If the Seed is 0, a random seed is selected to omit blocks.
- `pivot` (`mgl32.Vec3`)：Pivot is the pivot around which the structure may be rotated.

## 91 · ShowStoreOffer · 商店商品显示

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：打开商店商品界面；不属于机器人游戏命令执行。
- 本内核原始说明：ShowStoreOffer is sent by the server to show a Marketplace store offer to a player. It opens a window client-side that displays the item. The ShowStoreOffer packet only works on the partnered servers: Servers that are not partnered will not have a store buttons show up in the in-game pause menu and will, as a result, not be able to open store offers on the client side. Sending the packet does therefore not work when using a proxy that is not connected to with the domain of one of the partnered servers.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `offer_id` | `string` | OfferID is a string that identifies the offer for which a window should be opened. ID should be in the format of a UUID, however it can be left empty if StoreOfferTypeServerPage is used. |
| `type` | `uint8` | Type is the type of the store offer that is being shown to the player. It is one of the constants that may be found above. |

## 92 · PurchaseReceipt · 购买收据

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：PurchaseReceipt is sent by the client to the server to notify the server it purchased an item from the Marketplace store that was offered by the server. The packet is only used for partnered servers.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `receipts` | `[]string` | Receipts is a list of receipts, or proofs of purchases, for the offers that have been purchased by the player. |

## 93 · PlayerSkin · 玩家皮肤

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：PlayerSkin is sent by the client to the server when it updates its own skin using the in-game skin picker. It is relayed by the server, or sent if the server changes the skin of a player on its own accord. Note that the packet can only be sent for players that are in the player list at the time of sending.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `uuid` | `uuid.UUID` | UUID is the UUID of the player as sent in the Login packet when the client joined the server. It must match this UUID exactly for the skin to show up on the player. |
| `skin` | `protocol.Skin` | Skin is the new skin to be applied on the player with the UUID in the field above. The skin, including its animations, will be shown after sending it. |
| `new_skin_name` | `string` | NewSkinName no longer has a function: The field can be left empty at all times. |
| `old_skin_name` | `string` | OldSkinName no longer has a function: The field can be left empty at all times. |

### skin 嵌套字段

- `skin_id` (`string`)：SkinID is a unique ID produced for the skin, for example 'c18e65aa-7b21-4637-9b63-8ad63622ef01_Alex' for the default Alex skin.
- `play_fab_id` (`string`)：PlayFabID is the PlayFab ID produced for the skin. PlayFab is the company that hosts the Marketplace, skins and other related features from the game. This ID is the ID of the skin used to store the skin inside of PlayFab.
- `skin_resource_patch` (`[]byte`)：SkinResourcePatch is a JSON encoded object holding some fields that point to the geometry that the skin has. The JSON object that this holds specifies the way that the geometry of animations and the default skin of the player are combined.
- `skin_data` (`[]byte`)：SkinData is a byte slice of SkinImageWidth * SkinImageHeight bytes. It is an RGBA ordered byte representation of the skin pixels.
- `animations` (`[]SkinAnimation`)：Animations is a list of all animations that the skin has.
- `animations.image_data` (`[]byte`)：ImageData is a byte slice of ImageWidth * ImageHeight bytes. It is an RGBA ordered byte representation of the animation image pixels. The ImageData contains FrameCount images in it, which each represent one stage of the animation. The actual part of the skin that this field holds depends on the AnimationType, where SkinAnimationHead holds only the head and its hat, whereas the other animations hold the entire body of the skin.
- `animations.animation_type` (`uint32`)：AnimationType is the type of the animation, which is one of the types found above. The data that ImageData contains depends on this type.
- `animations.frame_count` (`float32`)：FrameCount is the amount of frames that the skin animation holds. The number of frames here is the amount of images that may be found in the ImageData field.
- `animations.expression_type` (`uint32`)：ExpressionType is the type of expression made by the skin, which is one the types found above.
- `cape_data` (`[]byte`)：CapeData is a byte slice of 64*32*4 bytes. It is a RGBA ordered byte representation of the cape colours, much like the SkinData.
- `skin_geometry` (`[]byte`)：SkinGeometry is a JSON encoded structure of the geometry data of a skin, containing properties such as bones, uv, pivot etc.
- `animation_data` (`[]byte`)：TODO: Find out what value AnimationData holds and when it does hold something.
- `geometry_data_engine_version` (`[]byte`)：GeometryDataEngineVersion ...
- `premium_skin` (`bool`)：PremiumSkin specifies if this is a skin that was purchased from the marketplace.
- `persona_skin` (`bool`)：PersonaSkin specifies if this is a skin that was created using the in-game skin creator.
- `persona_cape_on_classic_skin` (`bool`)：PersonaCapeOnClassicSkin specifies if the skin had a Persona cape (in-game skin creator cape) equipped on a classic skin.
- `primary_user` (`bool`)：PrimaryUser ...
- `cape_id` (`string`)：CapeID is a unique identifier that identifies the cape. It usually holds a UUID in it.
- `full_id` (`string`)：FullID is an ID that represents the skin in full. The actual functionality is unknown: The client does not seem to send a value for this.
- `skin_colour` (`string`)：SkinColour is a hex representation (including #) of the base colour of the skin. An example of the colour sent here is '#b37b62'.
- `arm_size` (`string`)：ArmSize is the size of the arms of the player's model. This is either 'wide' (generally for male skins) or 'slim' (generally for female skins).
- `persona_pieces` (`[]PersonaPiece`)：PersonaPieces is a list of all persona pieces that the skin is composed of.
- `persona_pieces.piece_id` (`string`)：PieceId is a UUID that identifies the piece itself, which is unique for each separate piece.
- `persona_pieces.piece_type` (`string`)：PieceType holds the type of the piece. Several types I was able to find immediately are listed below. - persona_skeleton - persona_body - persona_skin - persona_bottom - persona_feet - persona_top - persona_mouth - persona_hair - persona_eyes - persona_facial_hair
- `persona_pieces.pack_id` (`string`)：PackID is a UUID that identifies the pack that the persona piece belongs to.
- `persona_pieces.default` (`bool`)：Default specifies if the piece is one of the default pieces. This is true when the piece is one of those that a Steve or Alex skin have.
- `persona_pieces.product_id` (`string`)：ProductID is a UUID that identifies the piece when it comes to purchases. It is empty for pieces that have the 'Default' field set to true.
- `piece_tint_colours` (`[]PersonaPieceTintColour`)：PieceTintColours is a list of specific tint colours for (some of) the persona pieces found in the list above.
- `piece_tint_colours.piece_type` (`string`)：PieceType is the type of the persona skin piece that this tint colour concerns. The piece type must always be present in the persona pieces list, but not each piece type has a tint colour sent. Pieces that do have a tint colour that I was able to find immediately are listed below. - persona_mouth - persona_eyes - persona_hair
- `piece_tint_colours.colours` (`[]string`)：Colours is a list four colours written in hex notation (note, that unlike the SkinColour field in the ClientData struct, this is actually ARGB, not just RGB). The colours refer to different parts of the skin piece. The 'persona_eyes' may have the following colours: ["#ffa12722","#ff2f1f0f","#ff3aafd9","#0"] The first hex colour represents the tint colour of the iris, the second hex colour represents the eyebrows and the third represents the sclera. The fourth is #0 because there are only 3 parts of the persona_eyes skin piece.
- `trusted` (`bool`)：Trusted specifies if the skin is 'trusted'. No code should rely on this field, as any proxy or client can easily change it.
- `override_appearance` (`bool`)：OverrideAppearance specifies if the skin should override the player's skin that is equipped client-side. When false, the client will reject the skin and continue to use the skin that the player has equipped.

## 94 · SubClientLogin · 子客户端登录

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：SubClientLogin is sent when a sub-client joins the server while another client is already connected to it. The packet is sent as a result of split-screen game play, and allows up to four players to play using the same network connection. After an initial Login packet from the 'main' client, each sub-client that connects sends a SubClientLogin to request their own login.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `connection_request` | `[]uint8` | ConnectionRequest is a string containing information about the player and JWTs that may be used to verify if the player is connected to XBOX Live. The connection request also contains the necessary client public key to initiate encryption. The ConnectionRequest in this packet is identical to the one found in the Login packet. |

## 95 · AutomationClientConnect · 自动化客户端连接

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：教育/自动化场景连接信息，不能当作通用批量命令接口。
- 本内核原始说明：AutomationClientConnect is used to make the client connect to a websocket server. This websocket server has the ability to execute commands on the behalf of the client and it can listen for certain events fired by the client.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `server_uri` | `string` | ServerURI is the URI to make the client connect to. It can be, for example, 'localhost:8000/ws' to connect to a websocket server on the localhost at port 8000. |

## 96 · SetLastHurtBy · 最近受伤来源类型

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：同步最近伤害来源的实体类型，单独不能确认具体击杀玩家。
- 本内核原始说明：SetLastHurtBy is sent by the server to let the client know what entity type it was last hurt by. At this moment, the packet is useless and should not be used. There is no behaviour that depends on if this packet is sent or not.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_type` | `int32` | EntityType is the numerical type of the entity that the player was last hurt by. |

## 97 · BookEdit · 书本编辑

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：提交书页或签名修改，服务器验证库存物品与权限。
- 本内核原始说明：BookEdit is sent by the client when it edits a book. It is sent each time a modification was made and the player stops its typing 'session', rather than simply after closing the book.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `action_type` | `uint8` | 动作枚举，具体编号使用本内核定义。 ActionType is the type of the book edit action. The data obtained depends on what type this is. The action type is one of the constants above. |
| `inventory_slot` | `uint8` | InventorySlot is the slot in which the book that was edited may be found. Typically, the server should check if this slot matches the held item slot of the player. |
| `page_number` | `uint8` | PageNumber is the number of the page that the book edit action concerns. It applies for all actions but the BookActionSign. In BookActionSwapPages, it is one of the pages that was swapped. |
| `secondary_page_number` | `uint8` | SecondaryPageNumber is the page number of the second page that the action concerned. It is only set for the BookActionSwapPages action, in which case it is the other page that is swapped. |
| `text` | `string` | Text is the text that was written in a particular page of the book. It applies for the BookActionAddPage and BookActionReplacePage only. |
| `photo_name` | `string` | PhotoName is the name of the photo on the page in the book. It applies for the BookActionAddPage and BookActionReplacePage only. Unfortunately, the functionality of this field was removed from the default Minecraft Bedrock Edition. It is still available on Education Edition. |
| `title` | `string` | Title is the title that the player has given the book. It applies only for the BookActionSign action. |
| `author` | `string` | Author is the author that the player has given the book. It applies only for the BookActionSign action. Note that the author may be freely changed, so no assumptions can be made on if the author is actually the name of a player. |
| `xuid` | `string` | XUID is the XBOX Live User ID of the player that edited the book. The field is rather pointless, as the server is already aware of the XUID of the player anyway. |

## 98 · NPCRequest · 非玩家角色交互请求

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：NPC交互/动作请求，具体动作须按服务端支持。
- 本内核原始说明：NPCRequest is sent by the client when it interacts with an NPC. The packet is specifically made for Education Edition, where NPCs are available to use.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the NPC entity that the player interacted with. It is the same as sent by the server when spawning the entity. |
| `request_type` | `uint8` | RequestType is the type of the request, which depends on the permission that the player has. It will be either a type that indicates that the NPC should show its dialog, or that it should open the editing window. |
| `command_string` | `string` | CommandString is the command string set in the NPC. It may consist of multiple commands, depending on what the player set in it. |
| `action_type` | `uint8` | 动作枚举，具体编号使用本内核定义。 ActionType is the type of the action to execute. |
| `scene_name` | `string` | SceneName is the name of the scene. This can be left empty to specify the last scene that the player was sent. |

## 99 · PhotoTransfer · 照片转移

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：PhotoTransfer is sent by the server to transfer a photo (image) file to the client. It is typically used to transfer photos so that the client can display it in a portfolio in Education Edition. While previously usable in the default Bedrock Edition, the displaying of photos in books was disabled and the packet now has little use anymore.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `photo_name` | `string` | PhotoName is the name of the photo to transfer. It is the exact file name that the client will download the photo as, including the extension of the file. |
| `photo_data` | `[]uint8` | PhotoData is the raw data of the photo image. The format of this data may vary: Formats such as JPEG or PNG work, as long as PhotoName has the correct extension. |
| `book_id` | `string` | BookID is the ID of the book that the photo is associated with. If the PhotoName in a book with this ID is set to PhotoName, it will display the photo (provided Education Edition is used). The photo image is downloaded to a sub-folder with this book ID. |
| `photo_type` | `uint8` | PhotoType is one of the three photo types above. |
| `source_type` | `uint8` | SourceType is the source photo type. It is one of the three photo types above. |
| `owner_entity_unique_id` | `int64` | OwnerEntityUniqueID is the entity unique ID of the photo's owner. |
| `new_photo_name` | `string` | NewPhotoName is the new name of the photo. |

## 100 · ModalFormRequest · 表单显示请求

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：服务端发送表单JSON与ID，可解析菜单内容。
- 本内核原始说明：ModalFormRequest is sent by the server to make the client open a form. This form may be either a modal form which has two options, a menu form for a selection of options and a custom form for properties.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `form_id` | `uint32` | FormID is an ID used to identify the form. The ID is saved by the client and sent back when the player submits the form, so that the server can identify which form was submitted. |
| `form_data` | `[]uint8` | FormData is a JSON encoded object of form data. The content of the object differs, depending on the type of the form sent, which is also set in the JSON. |

## 101 · ModalFormResponse · 表单填写响应

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：客户端按表单ID提交回答或取消，服务器决定结果。
- 本内核原始说明：ModalFormResponse is sent by the client in response to a ModalFormRequest, after the player has submitted the form sent. It contains the options/properties selected by the player, or a JSON encoded 'null' if the form was closed by clicking the X at the top right corner of the form.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `form_id` | `uint32` | FormID is the form ID of the form the client has responded to. It is the same as the ID sent in the ModalFormRequest, and may be used to identify which form was submitted. |
| `response_data` | `protocol.Optional[[]uint8]` | ResponseData is a JSON encoded value representing the response of the player. For a modal form, the response is either true or false, for a menu form, the response is an integer specifying the index of the button clicked, and for a custom form, the response is an array containing a value for each element. |
| `cancel_reason` | `protocol.Optional[uint8]` | CancelReason represents the reason why the form was cancelled. It is one of the constants above. |

## 102 · ServerSettingsRequest · 服务器设置请求

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ServerSettingsRequest is sent by the client to request the settings specific to the server. These settings are shown in a separate tab client-side, and have the same structure as a custom form.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|

## 103 · ServerSettingsResponse · 服务器设置响应

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ServerSettingsResponse is optionally sent by the server in response to a ServerSettingsRequest from the client. It is structured the same as a ModalFormRequest packet, and if filled out correctly, will show a specific tab for the server in the settings of the client. A ModalFormResponse packet is sent by the client in response to a ServerSettingsResponse, when the client fills out the settings and closes the settings again.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `form_id` | `uint32` | FormID is an ID used to identify the form. The ID is saved by the client and sent back when the player submits the form, so that the server can identify which form was submitted. |
| `form_data` | `[]uint8` | FormData is a JSON encoded object of form data. The content of the object differs, depending on the type of the form sent, which is also set in the JSON. |

## 104 · ShowProfile · 玩家档案显示

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：显示指定玩家档案，不能等同完整账号资料查询。
- 本内核原始说明：ShowProfile is sent by the server to show the XBOX Live profile of one player to another.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `xuid` | `string` | XUID is the XBOX Live User ID of the player whose profile should be shown to the player. If it is not a valid XUID, the client ignores the packet. |

## 105 · SetDefaultGameType · 默认游戏模式

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：同步世界默认游戏模式。
- 本内核原始说明：SetDefaultGameType is sent by the client when it toggles the default game type in the settings UI, and is sent by the server when it actually changes the default game type, resulting in the toggle being changed in the settings UI.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `game_type` | `int32` | GameType is the new game type that is set. When sent by the client, this is the requested new default game type. |

## 106 · RemoveObjective · 移除目标

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：RemoveObjective is sent by the server to remove a scoreboard objective. It is used to stop showing a scoreboard to a player.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `objective_name` | `string` | ObjectiveName is the name of the objective that the scoreboard currently active has. This name must be identical to the one sent in the SetDisplayObjective packet. |

## 107 · SetDisplayObjective · 计分板显示目标

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：目标名称、标题、显示位置与排序，用于计分板界面解析。
- 本内核原始说明：SetDisplayObjective is sent by the server to display an object as a scoreboard to the player. Once sent, it should be followed up by a SetScore packet to set the lines of the packet.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `display_slot` | `string` | DisplaySlot is the slot in which the scoreboard should be displayed. Available options can be found in the constants above. |
| `objective_name` | `string` | ObjectiveName is the name of the objective that the scoreboard displays. Filling out a random unique value for this field works: It is not displayed in the scoreboard. |
| `display_name` | `string` | DisplayName is the name, or title, that is displayed at the top of the scoreboard. |
| `criteria_name` | `string` | CriteriaName is the name of the criteria that need to be fulfilled in order for the score to be increased. This can be any kind of string and does not show up client-side. |
| `sort_order` | `int32` | SortOrder is the order in which entries on the scoreboard should be sorted. It is one of the constants that may be found above. |

## 108 · SetScore · 计分板分数变化

- 分组：命令与计分板
- 方向：服务器→客户端
- 用法与边界：服务器选择同步的记分板分数；显示板外的全部分数仍可能需OP命令查询。
- 本内核原始说明：SetScore is sent by the server to send the contents of a scoreboard to the player. It may be used to either add, remove or edit entries on the scoreboard.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `action_type` | `uint8` | 动作枚举，具体编号使用本内核定义。 ActionType is the type of the action to execute upon the scoreboard with the entries that the packet has. If ActionType is ScoreboardActionModify, all entries will be added to the scoreboard if not yet present, or modified if already present. If set to ScoreboardActionRemove, all scoreboard entries set will be removed from the scoreboard. |
| `entries` | `[]protocol.ScoreboardEntry` | Entries is a list of all entries that the client should operate on. When modifying, it will add or modify all entries, whereas when removing, it will remove all entries. |

### entries 嵌套字段

- `entry_id` (`int64`)：EntryID is a unique identifier of this entry. Each entry that represents a different value should get its own entry ID. When modifying a scoreboard, entries that represent the same line should have the same entry ID.
- `objective_name` (`string`)：ObjectiveName is the name of the objective that this scoreboard entry is for. It must be identical to the one set in the SetDisplayObjective packet previously sent.
- `score` (`int32`)：Score is the score that the entry represents. Scoreboard entries are ordered using this score, so in order to get the scoreboard to be ordered as expected when trying to write non-score related text on a scoreboard, this score should be incremented for each entry.
- `identity_type` (`byte`)：IdentityType is the identity type of the scoreboard entry. The entry may represent an entity, player or a fake player, as the constants above indicate. In order to write plain text to the scoreboard, ScoreboardIdentityFakePlayer should always be used, in combination with the DisplayName field. A different identity type will use the name of the entity.
- `entity_unique_id` (`int64`)：EntityUniqueID is the unique ID of either the player or the entity represented by the scoreboard entry. This field is only used if IdentityType is either ScoreboardIdentityEntity or ScoreboardIdentityPlayer.
- `display_name` (`string`)：DisplayName is the custom name of the scoreboard entry. This field is only used if IdentityType is ScoreboardIdentityFakePlayer. If this identity type is not used, the name of the entity/player will be shown instead.

## 109 · LabTable · 实验台操作

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：教育版实验台动作；依赖对应玩法环境。
- 本内核原始说明：LabTable is sent by the client to let the server know it started a chemical reaction in Education Edition, and is sent by the server to other clients to show the effects. The packet is only functional if Education features are enabled.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `action_type` | `uint8` | 动作枚举，具体编号使用本内核定义。 ActionType is the type of the action that was executed. It is one of the constants above. Typically, only LabTableActionCombine is sent by the client, whereas LabTableActionReact is sent by the server. |
| `position` | `protocol.BlockPos` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position at which the lab table used was located. |
| `reaction_type` | `uint8` | ReactionType is the type of the reaction that took place as a result of the items put into the lab table. The reaction type can be either that of an item or a particle, depending on whatever the result was of the reaction. |

## 110 · UpdateBlockSynced · 同步方块变更

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：携带同步实体信息的方块变化，不直接证明破坏者身份。
- 本内核原始说明：UpdateBlockSynced is sent by the server to synchronise the falling of a falling block entity with the transitioning back and forth from and to a solid block. It is used to prevent the entity from flickering, and is used in places such as the pushing of blocks with pistons.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `position` | `protocol.BlockPos` | 位置坐标，维度与有效性按本包上下文确定。 Position is the block position at which a block is updated. |
| `new_block_runtime_id` | `uint32` | NewBlockRuntimeID is the runtime ID of the block that is placed at Position after sending the packet to the client. |
| `flags` | `uint32` | Flags is a combination of flags that specify the way the block is updated client-side. It is a combination of the flags above, but typically sending only the BlockUpdateNetwork flag is sufficient. |
| `layer` | `uint32` | Layer is the world layer on which the block is updated. For most blocks, this is the first layer, as that layer is the default layer to place blocks on, but for blocks inside of each other, this differs. |
| `entity_unique_id` | `uint64` | 实体唯一ID，与运行ID不同。 EntityUniqueID is the unique ID of the falling block entity that the block transitions to or that the entity transitions from. Note that for both possible values for TransitionType, the EntityUniqueID should point to the falling block entity involved. |
| `transition_type` | `uint64` | TransitionType is the type of the transition that happened. It is either BlockToEntityTransition, when a block placed becomes a falling entity, or EntityToBlockTransition, when a falling entity hits the ground and becomes a solid block again. |

## 111 · MoveActorDelta · 实体差量位置

- 分组：移动与输入
- 方向：服务器→客户端
- 用法与边界：只更新flags指定轴，不能将未出现的坐标轴当0。
- 本内核原始说明：MoveActorDelta is sent by the server to move an entity. The packet is specifically optimised to save as much space as possible, by only writing non-zero fields. As of 1.16.100, this packet no longer actually contains any deltas.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `flags` | `uint16` | Flags is a list of flags that specify what data is in the packet. |
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the entity that is being moved. The packet works provided a non-player entity with this runtime ID is present. |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the new position that the entity was moved to. |
| `rotation` | `mgl32.Vec3` | Rotation is the new absolute rotation. Unlike the position, it is not actually a delta. If any of the values of this rotation are not sent, these values are 0 and no flag for them is present. |

## 112 · SetScoreboardIdentity · 计分板身份绑定

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：将计分条目和实体身份关联，配合SetScore解释分数。
- 本内核原始说明：SetScoreboardIdentity is sent by the server to change the identity type of one of the entries on a scoreboard. This is used to change, for example, an entry pointing to a player, to a fake player when it leaves the server, and to change it back to a real player when it joins again. In non-vanilla situations, the packet is quite useless.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `action_type` | `uint8` | 动作枚举，具体编号使用本内核定义。 ActionType is the type of the action to execute. The action is either ScoreboardIdentityActionRegister to associate an identity with the entry, or ScoreboardIdentityActionClear to remove associations with an entity. |
| `entries` | `[]protocol.ScoreboardIdentityEntry` | Entries is a list of all entries in the packet. Each of these entries points to one of the entries on a scoreboard. Depending on ActionType, their identity will either be registered or cleared. |

### entries 嵌套字段

- `entry_id` (`int64`)：EntryID is the unique identifier of the entry that the identity should be associated with, or that associations should be cleared from.
- `entity_unique_id` (`int64`)：EntityUniqueID is the unique ID that the entry should be associated with. It is empty if the SetScoreboardIdentity packet is sent to remove associations with identities.

## 113 · SetLocalPlayerAsInitialised · 本机初始化完成

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：客户端通知玩家初始化已完成，由连接流程负责。
- 本内核原始说明：SetLocalPlayerAsInitialised is sent by the client in response to a PlayStatus packet with the status set to 3. The packet marks the moment at which the client is fully initialised and can receive any packet without discarding it.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the entity runtime ID the player was assigned earlier in the login sequence in the StartGame packet. |

## 114 · UpdateSoftEnum · 更新软枚举

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：UpdateSoftEnum is sent by the server to update a soft enum, also known as a dynamic enum, previously sent in the AvailableCommands packet. It is sent whenever the enum should get new options or when some of its options should be removed. The UpdateSoftEnum packet will apply for enums that have been set in the AvailableCommands packet with the 'Dynamic' field of the CommandEnum set to true.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `enum_type` | `string` | EnumType is the type of the enum. This type must be identical to the one set in the AvailableCommands packet, because the client uses this to recognise which enum to update. |
| `options` | `[]string` | Options is a list of options that should be updated. Depending on the ActionType field, either these options will be added to the enum, the enum options will be set to these options or all of these options will be removed from the enum. |
| `action_type` | `uint8` | 动作枚举，具体编号使用本内核定义。 ActionType is the type of the action to execute on the enum. The Options field has a different result, depending on what ActionType is used. |

## 115 · NetworkStackLatency · 网络栈延迟

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：NetworkStackLatency is sent by the server (and the client, on development builds) to measure the latency over the entire Minecraft stack, rather than the RakNet latency. It has other usages too, such as the ability to be used as some kind of acknowledgement packet, to know when the client has received a certain other packet.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `timestamp` | `int64` | Timestamp is the timestamp of the network stack latency packet. The client will, if NeedsResponse is set to true, send a NetworkStackLatency packet with this same timestamp packet in response. |
| `needs_response` | `bool` | NeedsResponse specifies if the sending side of this packet wants a response to the packet, meaning that the other side should send a NetworkStackLatency packet back. |

## 117 · ScriptCustomEvent · 脚本自定义事件

- 分组：网易与扩展
- 方向：双向取决于服务端
- 用法与边界：必须服务端有接收/发送实现；客户端不能单方面创建新的服务端能力。
- 本内核原始说明：ScriptCustomEvent is sent by both the client and the server. It is a way to let scripts communicate with the server, so that the client can let the server know it triggered an event, or the other way around. ScriptCustomEvent is deprecated as of 1.20.10.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `event_name` | `string` | EventName is the name of the event. The script and the server will use this event name to identify the data that is sent. |
| `event_data` | `[]uint8` | EventData is the data of the event. This data is typically a JSON encoded string, that the script is able to encode and decode too. |

## 118 · SpawnParticleEffect · 出生粒子效果

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：SpawnParticleEffect is sent by the server to spawn a particle effect client-side. Unlike other packets that result in the appearing of particles, this packet can show particles that are not hardcoded in the client. They can be added and changed through behaviour packs to implement custom particles.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `dimension` | `uint8` | Dimension is the dimension that the particle is spawned in. Its exact usage is not clear, as the dimension has no direct effect on the particle. |
| `entity_unique_id` | `int64` | 实体唯一ID，与运行ID不同。 EntityUniqueID is the unique ID of the entity that the spawned particle may be attached to. If this ID is not -1, the Position below will be interpreted as relative to the position of the entity associated with this unique ID. |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position that the particle should be spawned at. If the position is too far away from the player, it will not show up. If EntityUniqueID is not -1, the position will be relative to the position of the entity. |
| `particle_name` | `string` | ParticleName is the name of the particle that should be shown. This name may point to a particle effect that is built-in, or to one implemented by behaviour packs. |
| `mo_lang_variables` | `protocol.Optional[[]uint8]` | MoLangVariables is an encoded JSON map of MoLang variables that may be applicable to the particle spawn. This can just be left empty in most cases. |

## 119 · AvailableActorIdentifiers · 实体类型注册表

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：服务端下发可用实体标识，不表示这些实体已生成。
- 本内核原始说明：AvailableActorIdentifiers is sent by the server at the start of the game to let the client know all entities that are available on the server.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `serialised_entity_identifiers` | `[]uint8` | SerialisedEntityIdentifiers is a network NBT serialised compound of all entity identifiers that are available in the server. |

## 121 · NetworkChunkPublisherUpdate · 区块发布中心更新

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：服务器的区块中心、半径与已保存区块信息。
- 本内核原始说明：NetworkChunkPublisherUpdate is sent by the server to change the point around which chunks are and remain loaded. This is useful for mini-game servers, where only one area is ever loaded, in which case the NetworkChunkPublisherUpdate packet can be sent in the middle of it, so that no chunks ever need to be additionally sent during the course of the game. In reality, the packet is not extraordinarily useful, and most servers just send it constantly at the position of the player. If the packet is not sent at all, no chunks will be shown to the player, regardless of where they are sent.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `position` | `protocol.BlockPos` | 位置坐标，维度与有效性按本包上下文确定。 Position is the block position around which chunks loaded will remain shown to the client. Most servers set this position to the position of the player itself. |
| `radius` | `uint32` | Radius is the radius in blocks around Position that chunks sent show up in and will remain loaded in. Unlike the RequestChunkRadius and ChunkRadiusUpdated packets, this radius is in blocks rather than chunks, so the chunk radius needs to be multiplied by 16. (Or shifted to the left by 4.) |
| `saved_chunks` | `[]protocol.ChunkPos` | SavedChunks ... TODO: Figure out what this field is used for. |

## 122 · BiomeDefinitionList · 生物群系定义表

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：服务端同步生物群系数据，供世界渲染/识别。
- 本内核原始说明：BiomeDefinitionList is sent by the server to let the client know all biomes that are available and implemented on the server side. When enabled, it also includes information for the client to accurately recreate the server-side generation in vanilla worlds/servers for increased performance.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `biome_definitions` | `[]protocol.BiomeDefinition` | BiomeDefinitions is a list of biomes that are available on the server. |
| `string_list` | `[]string` | StringList is a makeshift dictionary implementation Mojang created to try and reduce the size of the overall packet. It is a list of common strings that are used in the biome definitions, such as biome names, float values or query expressions. |

### biome_definitions 嵌套字段

- `name_index` (`int16`)：NameIndex represents the index of the biome name in the string list.
- `biome_id` (`Optional[uint16]`)：BiomeID is the biome ID. This is optional and can be empty.
- `temperature` (`float32`)：Temperature is the temperature of the biome, used for weather, biome behaviours and sky colour.
- `downfall` (`float32`)：Downfall is the amount that precipitation affects colours and block changes.
- `red_spore_density` (`float32`)：RedSporeDensity is the density of red spore precipitation visuals.
- `blue_spore_density` (`float32`)：BlueSporeDensity is the density of blue spore precipitation visuals.
- `ash_density` (`float32`)：AshDensity is the density of ash precipitation visuals.
- `white_ash_density` (`float32`)：WhiteAshDensity is the density of white ash precipitation visuals.
- `depth` (`float32`)：Depth ...
- `scale` (`float32`)：Scale ...
- `map_water_colour` (`int32`)：MapWaterColour is an ARGB value for the water colour on maps in the biome.
- `rain` (`bool`)：Rain is true if the biome has rain, false if it is a dry biome.
- `dimension` (`int32`)：PhoenixBuilder specific fields. Author: Happy2018new  Netease
- `vanilla` (`string`)：
- `tags` (`Optional[[]uint16]`)：Tags are a list of indices of tags in the string list. These are used to group biomes together for biome generation and other purposes.
- `chunk_generation` (`Optional[BiomeChunkGeneration]`)：ChunkGeneration is optional information to assist in client-side chunk generation. Almost all servers can and should leave this empty to greatly reduce the size of this packet. Only BDS and servers which *exactly* match the vanilla chunk generation can benefit from this.

## 123 · LevelSoundEvent · 世界声音事件

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：LevelSoundEvent is sent by the server to make any kind of built-in sound heard to a player. It is sent to, for example, play a stepping sound or a shear sound. The packet is also sent by the client, in which case it could be forwarded by the server to the other players online. If possible, the packets from the client should be ignored however, and the server should play them on its own accord.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `sound_type` | `uint32` | SoundType is the type of the sound to play. It is one of the constants above. Some of the sound types require additional data, which is set in the EventData field. |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position of the sound event. The player will be able to hear the direction of the sound based on what position is sent here. |
| `extra_data` | `int32` | ExtraData is a packed integer that some sound types use to provide extra data. An example of this is the note sound, which is composed of a pitch and an instrument type. |
| `entity_type` | `string` | EntityType is the string entity type of the entity that emitted the sound, for example 'minecraft:skeleton'. Some sound types use this entity type for additional data. |
| `baby_mob` | `bool` | BabyMob specifies if the sound should be that of a baby mob. It is most notably used for parrot imitations, which will change based on if this field is set to true or not. |
| `disable_relative_volume` | `bool` | DisableRelativeVolume specifies if the sound should be played relatively or not. If set to true, the sound will have full volume, regardless of where the Position is, whereas if set to false, the sound's volume will be based on the distance to Position. |
| `entity_unique_id` | `int64` | 实体唯一ID，与运行ID不同。 EntityUniqueID is the unique ID of a source entity. The unique ID is a value that remains consistent across different sessions of the same world, but most servers simply fill the runtime ID of the entity out for this field. |

## 124 · LevelEventGeneric · 世界事件通用

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：LevelEventGeneric is sent by the server to send a 'generic' level event to the client. This packet sends an NBT serialised object and may for that reason be used for any event holding additional data.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `event_id` | `int32` | EventID is a unique identifier that identifies the event called. The data that follows has fields in the NBT depending on what event it is. |
| `serialised_event_data` | `[]uint8` | SerialisedEventData is a network little endian serialised object of event data, with fields that vary depending on EventID. Unlike many other NBT structures, this data is not actually in a compound but just loosely floating NBT tags. To decode using the nbt package, you would need to append 0x0a00 at the start (compound id and name length) and add 0x00 at the end, to manually wrap it in a compound. Likewise, you would have to remove these bytes when encoding. Example of the resulting data with an EventID of 2026: TAG_Compound({ 'pos15x': TAG_Float(198), 'pos11x': TAG_Float(201), 'pos6y': TAG_Float(65), 'pos13y': TAG_Float(64), 'pos17z': TAG_Float(36), 'pos8y': TAG_Float(65), 'originY': TAG_Float(65.06125), 'pos10z': TAG_Float(37), 'pos13x': TAG_Float(201), 'pos7y': TAG_Float(65), 'pos9x': TAG_Float(203), 'pos11y': TAG_Float(64), 'pos15y': TAG_Float(65), 'pos15z': TAG_Float(40), 'pos7z': TAG_Float(41), 'pos8x': TAG_Float(198), 'pos13z': TAG_Float(40), 'pos1z': TAG_Float(37), 'pos6z': TAG_Float(42), 'size': TAG_Int(18), 'pos0x': TAG_Float(204), 'pos12x': TAG_Float(200), 'pos2x': TAG_Float(204), 'pos9z': TAG_Float(37), 'pos16y': TAG_Float(64), 'pos5x': TAG_Float(204), 'pos5y': TAG_Float(64), 'pos17x': TAG_Float(202), 'pos3y': TAG_Float(64), 'pos3z': TAG_Float(36), 'radius': TAG_Float(4), 'pos0z': TAG_Float(38), 'pos4z': TAG_Float(36), 'pos8z': TAG_Float(38), 'pos1x': TAG_Float(204), 'pos0y': TAG_Float(64), 'pos14z': TAG_Float(39), 'pos16z': TAG_Float(40), 'pos2y': TAG_Float(63), 'pos6x': TAG_Float(203), 'pos10x': TAG_Float(205), 'pos12y': TAG_Float(64), 'pos1y': TAG_Float(64), 'pos14x': TAG_Float(200), 'pos3x': TAG_Float(204), 'pos9y': TAG_Float(64), 'pos4y': TAG_Float(63), 'pos10y': TAG_Float(63), 'pos12z': TAG_Float(38), 'pos16x': TAG_Float(202), 'originX': TAG_Float(202.48654), 'pos14y': TAG_Float(62), 'pos17y': TAG_Float(62), 'pos5z': TAG_Float(35), 'pos4x': TAG_Float(204), 'pos7x': TAG_Float(203), 'originZ': TAG_Float(38.297028), 'pos11z': TAG_Float(38), 'pos2z': TAG_Float(39), }) The 'originX', 'originY' and 'originZ' fields are present in every event and serve as a replacement for a Position field in this packet. |

## 125 · LecternUpdate · 讲台页面更新

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：讲台位置和页码变化，依赖已加载区域。
- 本内核原始说明：LecternUpdate is sent by the client to update the server on which page was opened in a book on a lectern, or if the book should be removed from it.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `page` | `uint8` | Page is the page number in the book that was opened by the player on the lectern. |
| `page_count` | `uint8` | PageCount is the number of pages that the book opened in the lectern has. |
| `position` | `protocol.BlockPos` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position of the lectern that was updated. If no lectern is at the block position, the packet should be ignored. |

## 129 · ClientCacheStatus · 客户端缓存状态

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ClientCacheStatus is sent by the client to the server at the start of the game. It is sent to let the server know if it supports the client-side blob cache. Clients such as Nintendo Switch do not support the cache, and attempting to use it anyway will fail.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `enabled` | `bool` | Enabled specifies if the blob cache is enabled. If false, the server should not attempt to use the blob cache. If true, it may do so, but it may also choose not to use it. |

## 130 · OnScreenTextureAnimation · 屏幕纹理动画

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：客户端屏幕效果；方块视频功能不使用它替代真实方块。
- 本内核原始说明：OnScreenTextureAnimation is sent by the server to show a certain animation on the screen of the player. The packet is used, as an example, for when a raid is triggered and when a raid is defeated.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `animation_type` | `int32` | AnimationType is the type of the animation to show. The packet provides no further extra data to allow modifying the duration or other properties of the animation. |

## 131 · MapCreateLockedCopy · 创建锁定地图副本

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：请求锁定地图副本，须服务端认可地图物品和ID。
- 本内核原始说明：MapCreateLockedCopy is sent by the client to create a locked copy of one map into another map. In vanilla, it is used in the cartography table to create a map that is locked and cannot be modified.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `original_map_id` | `int64` | OriginalMapID is the ID of the map that is being copied. The locked copy will obtain all content that is visible on this map, except the content will not change. |
| `new_map_id` | `int64` | NewMapID is the ID of the map that holds the locked copy of the map that OriginalMapID points to. Its contents will be impossible to change. |

## 132 · StructureTemplateDataRequest · 结构模板数据请求

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：请求结构区域或模板数据；建筑导出相关，不是第三方网页转换。
- 本内核原始说明：StructureTemplateDataRequest is sent by the client to request data of a structure.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `structure_name` | `string` | StructureName is the name of the structure that was set in the structure block's UI. This is the name used to export the structure to a file. |
| `position` | `protocol.BlockPos` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position of the structure block that has its template data requested. |
| `settings` | `protocol.StructureSettings` | Settings is a struct of settings that should be used for exporting the structure. These settings are identical to the last sent in the StructureBlockUpdate packet by the client. |
| `request_type` | `uint8` | RequestType specifies the type of template data request that the player sent. It is one of the constants found above. |

### settings 嵌套字段

- `palette_name` (`string`)：PaletteName is the name of the palette used in the structure. Currently, it seems that this field is always 'default'.
- `ignore_entities` (`bool`)：IgnoreEntities specifies if the structure should ignore entities or include them. If set to false, entities will also show up in the exported structure.
- `ignore_blocks` (`bool`)：IgnoreBlocks specifies if the structure should ignore blocks or include them. If set to false, blocks will show up in the exported structure.
- `allow_non_ticking_chunks` (`bool`)：AllowNonTickingChunks specifies if the structure should allow non-ticking chunks. If set to false, the structure will export non-ticking chunks.
- `size` (`BlockPos`)：Size is the size of the area that is about to be exported. The area exported will start at the Position + Offset, and will extend as far as Size specifies.
- `offset` (`BlockPos`)：Offset is the offset position that was set in the structure block. The area exported is offset by this position.
- `last_editing_player_unique_id` (`int64`)：LastEditingPlayerUniqueID is the unique ID of the player that last edited the structure block that these settings concern.
- `rotation` (`byte`)：Rotation is the rotation that the structure block should obtain. See the constants above for available options.
- `mirror` (`byte`)：Mirror specifies the way the structure should be mirrored. It is either no mirror at all, mirror on the x/z axis or both.
- `animation_mode` (`byte`)：AnimationMode ...
- `animation_duration` (`float32`)：AnimationDuration ...
- `integrity` (`float32`)：Integrity is usually 1, but may be set to a number between 0 and 1 to omit blocks randomly, using the Seed that follows.
- `seed` (`uint32`)：Seed is the seed used to omit blocks if Integrity is not equal to one. If the Seed is 0, a random seed is selected to omit blocks.
- `pivot` (`mgl32.Vec3`)：Pivot is the pivot around which the structure may be rotated.

## 133 · StructureTemplateDataResponse · 结构模板数据响应

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：服务器返回结构与NBT或失败信息，用于读取结果核对。
- 本内核原始说明：StructureTemplateDataResponse is sent by the server to send data of a structure to the client in response to a StructureTemplateDataRequest packet.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `structure_name` | `string` | StructureName is the name of the structure that was requested. This is the name used to export the structure to a file. |
| `success` | `bool` | Success specifies if a structure template was found by the StructureName that was sent in a StructureTemplateDataRequest packet. |
| `response_type` | `uint8` | ResponseType specifies the response type of the packet. This depends on the RequestType field sent in the StructureTemplateDataRequest packet and is one of the constants above. |
| `structure_template` | `map[string]interface {}` | StructureTemplate holds the data of the structure template. |

## 135 · ClientCacheBlobStatus · 客户端缓存块状态

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：通知缓存命中/缺失的blob，供区块缓存协议使用。
- 本内核原始说明：ClientCacheBlobStatus is part of the blob cache protocol. It is sent by the client to let the server know what blobs it needs and which blobs it already has, in an ACK type system.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `miss_hashes` | `[]uint64` | MissHashes is a list of blob hashes that the client does not have a blob available for. The server should send the blobs matching these hashes as soon as possible. |
| `hit_hashes` | `[]uint64` | HitHashes is a list of blob hashes that the client has a blob available for. The blobs hashes here mean that the client already has them: The server does not need to send the blobs anymore. |

## 136 · ClientCacheMissResponse · 缓存缺失数据响应

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：服务器补发客户端缺失的缓存数据块。
- 本内核原始说明：ClientCacheMissResponse is part of the blob cache protocol. It is sent by the server in response to a ClientCacheBlobStatus packet and contains the blob data of all blobs that the client acknowledged not to have yet.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `blobs` | `[]protocol.CacheBlob` | Blobs is a list of all blobs that the client sent misses for in the ClientCacheBlobStatus. These blobs hold the data of the blobs with the hashes they are matched with. |

### blobs 嵌套字段

- `hash` (`uint64`)：Hash is the hash of the blob. The hash is computed using xxHash, and must be deterministic for the same chunk data.
- `payload` (`[]byte`)：Payload is the data of the blob. When sent, the client will associate the Hash of the blob with the Payload in it.

## 137 · EducationSettings · 教育设置

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：EducationSettings is a packet sent by the server to update Minecraft: Education Edition related settings. It is unused by the normal base game.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `code_builder_default_uri` | `string` | CodeBuilderDefaultURI is the default URI that the code builder is ran on. Using this, a Code Builder program can make code directly affect the server. |
| `code_builder_title` | `string` | CodeBuilderTitle is the title of the code builder shown when connected to the CodeBuilderDefaultURI. |
| `can_resize_code_builder` | `bool` | CanResizeCodeBuilder specifies if clients connected to the world should be able to resize the code builder when it is opened. |
| `disable_legacy_title_bar` | `bool` | DisableLegacyTitleBar ... |
| `post_process_filter` | `string` | PostProcessFilter ... |
| `screenshot_border_path` | `string` | ScreenshotBorderPath ... |
| `can_modify_blocks` | `protocol.Optional[bool]` | CanModifyBlocks ... |
| `override_uri` | `protocol.Optional[string]` | OverrideURI ... |
| `has_quiz` | `bool` | HasQuiz specifies if the world has a quiz connected to it. |
| `external_link_settings` | `protocol.Optional[github.com/OmineDev/flowers-for-machines/core/minecraft/protocol.EducationExternalLinkSettings]` | ExternalLinkSettings ... |

## 138 · Emote · 表情

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：Emote is sent by both the server and the client. When the client sends an emote, it sends this packet to the server, after which the server will broadcast the packet to other players online.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the entity that sent the emote. When a player sends this packet, it has this field set as its own entity runtime ID. |
| `emote_length` | `uint32` | EmoteLength is the number of ticks that the emote lasts for. |
| `emote_id` | `string` | EmoteID is the ID of the emote to send. |
| `xuid` | `string` | XUID is the Xbox User ID of the player that sent the emote. It is only set when the emote is used by a player that is authenticated with Xbox Live. |
| `platform_id` | `string` | PlatformID is an identifier only set for particular platforms when using an emote (presumably only for Nintendo Switch). It is otherwise an empty string, and is used to decide which players are able to emote with each other. |
| `flags` | `uint8` | Flags is a combination of flags that change the way the Emote packet operates. When the server sends this packet to other players, EmoteFlagServerSide must be present. |

## 139 · MultiPlayerSettings · 多玩家设置

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：MultiPlayerSettings is sent by the client to update multi-player related settings server-side and sent back to online players by the server. The MultiPlayerSettings packet is a Minecraft: Education Edition packet. It has no functionality for the base game.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `action_type` | `int32` | 动作枚举，具体编号使用本内核定义。 ActionType is the action that should be done when this packet is sent. It is one of the constants that may be found above. |

## 140 · SettingsCommand · 设置命令无等待请求

- 分组：命令与计分板
- 方向：客户端→服务器
- 用法与边界：当前魔法Fast路径使用它，不逐条等待回包。提交顺序不证明任意长链同一个服务端Tick完成。
- 本内核原始说明：SettingsCommand is sent by the client when it changes a setting in the settings that results in the issuing of a command to the server, such as when Show Coordinates is enabled.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `command_line` | `string` | CommandLine is the full command line that was sent to the server as a result of the setting that the client changed. |
| `suppress_output` | `bool` | SuppressOutput specifies if the client requests the suppressing of the output of the command that was executed. Generally this is set to true, as the client won't need a message to confirm the output of the change. |

## 141 · AnvilDamage · 铁砧损耗

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：铁砧耐用度/损耗状态，不能通过旁观端控制任意铁砧。
- 本内核原始说明：AnvilDamage is sent by the client to request the dealing damage to an anvil. This packet is completely pointless and the server should never listen to it.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `damage` | `uint8` | Damage is the damage that the client requests to be dealt to the anvil. |
| `anvil_position` | `protocol.BlockPos` | AnvilPosition is the position in the world that the anvil can be found at. |

## 142 · CompletedUsingItem · 本机完成使用物品

- 分组：物品与容器
- 方向：按服务端具体同步
- 用法与边界：可观测被同步的完成动作；不是所有玩家消费物品全局事件。
- 本内核原始说明：CompletedUsingItem is sent by the server to tell the client that it should be done using the item it is currently using.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `used_item_id` | `int16` | UsedItemID is the item ID of the item that the client completed using. This should typically be the ID of the item held in the hand. |
| `use_method` | `int32` | UseMethod is the method of the using of the item that was completed. It is one of the constants that may be found above. |

## 143 · NetworkSettings · 网络设置

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：NetworkSettings is sent by the server to update a variety of network settings. These settings modify the way packets are sent over the network stack.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `compression_threshold` | `uint16` | CompressionThreshold is the minimum size of a packet that is compressed when sent. If the size of a packet is under this value, it is not compressed. When set to 0, all packets will be left uncompressed. |
| `compression_algorithm` | `uint16` | CompressionAlgorithm is the algorithm that is used to compress packets. |
| `client_throttle` | `bool` | ClientThrottle regulates whether the client should throttle players when exceeding of the threshold. Players outside threshold will not be ticked, improving performance on low-end devices. |
| `client_throttle_threshold` | `uint8` | ClientThrottleThreshold is the threshold for client throttling. If the number of players exceeds this value, the client will throttle players. |
| `client_throttle_scalar` | `float32` | ClientThrottleScalar is the scalar for client throttling. The scalar is the amount of players that are ticked when throttling is enabled. |

## 144 · PlayerAuthInput · 本机权威输入帧

- 分组：移动与输入
- 方向：客户端→服务器
- 用法与边界：移动、视角、tick、跳跃/潜行/疾跑等输入，含可选方块动作和库存事务。旁观机器人通常收不到他人的输入；由宿主心跳持续驱动。
- 本内核原始说明：PlayerAuthInput is sent by the client to allow for server authoritative movement. It is used to synchronise the player input with the position server-side. The client sends this packet when the ServerAuthoritativeMovementMode field in the StartGame packet is set to true, instead of the MovePlayer packet. The client will send this packet once every tick.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `pitch` | `float32` | 俯仰角。  |
| `yaw` | `float32` | 偏航角。  |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position holds the position that the player reports it has. |
| `move_vector` | `mgl32.Vec2` | MoveVector is a Vec2 that specifies the direction in which the player moved, as a combination of X/Z values which are created using the WASD/controller stick state. |
| `head_yaw` | `float32` | 头部偏航角。 HeadYaw is the horizontal rotation of the head that the player reports it has. |
| `input_data` | `protocol.Bitset` | InputData is a combination of bit flags that together specify the way the player moved last tick. It is a combination of the flags above. |
| `input_mode` | `uint32` | InputMode specifies the way that the client inputs data to the screen. It is one of the constants that may be found above. |
| `play_mode` | `uint32` | PlayMode specifies the way that the player is playing. The values it holds, which are rather random, may be found above. |
| `interaction_model` | `uint32` | InteractionModel is a constant representing the interaction model the player is using. It is one of the constants that may be found above. |
| `interact_pitch` | `float32` |  |
| `interact_yaw` | `float32` |  |
| `tick` | `uint64` | 输入/同步的游戏刻编号。 Tick is the server tick at which the packet was sent. It is used in relation to CorrectPlayerMovePrediction. |
| `delta` | `mgl32.Vec3` | Delta was the delta between the old and the new position. There isn't any practical use for this field as it can be calculated by the server itself. |
| `item_interaction_data` | `protocol.UseItemTransactionData` | ItemInteractionData is the transaction data if the InputData includes an item interaction. |
| `item_stack_request` | `protocol.ItemStackRequest` | ItemStackRequest is sent by the client to change an item in their inventory. |
| `block_actions` | `[]protocol.PlayerBlockAction` | BlockActions is a slice of block actions that the client has interacted with. |
| `vehicle_rotation` | `mgl32.Vec2` | VehicleRotation is the rotation of the vehicle that the player is in, if any. |
| `client_predicted_vehicle` | `int64` | ClientPredictedVehicle is the unique ID of the vehicle that the client predicts the player to be in. |
| `analogue_move_vector` | `mgl32.Vec2` | AnalogueMoveVector is a Vec2 that specifies the direction in which the player moved, as a combination of X/Z values which are created using an analogue input. |
| `camera_orientation` | `mgl32.Vec3` | CameraOrientation is the vector that represents the camera's forward direction which can be used to transform movement to be camera relative. |
| `raw_move_vector` | `mgl32.Vec2` | RawMoveVector is the value of MoveVector before it is affected by input permissions, sneaking/fly speeds and isn't normalised for analogue inputs. |
| `unknown1` | `float32` | PhoenixBuilder specific fields. Author: Liliya233  The following fields are all NetEase specific. |
| `unknown2` | `float32` |  |
| `unknown3` | `bool` |  |
| `unknown4` | `bool` |  |
| `unknown5` | `bool` |  |
| `unknown6` | `bool` |  |
| `unknown7` | `float32` |  |

### input_data 嵌套字段

- `size` (`int`)：
- `int` (`*big.Int`)：

### item_interaction_data 嵌套字段

- `legacy_request_id` (`int32`)：LegacyRequestID is an ID that is only non-zero at times when sent by the client. The server should always send 0 for this. When this field is not 0, the LegacySetItemSlots slice below will have values in it. LegacyRequestID ties in with the ItemStackResponse packet. If this field is non-0, the server should respond with an ItemStackResponse packet. Some inventory actions such as dropping an item out of the hotbar are still one using this packet, and the ItemStackResponse packet needs to tie in with it.
- `legacy_set_item_slots` (`[]LegacySetItemSlot`)：LegacySetItemSlots are only present if the LegacyRequestID is non-zero. These item slots inform the server of the slots that were changed during the inventory transaction, and the server should send back an ItemStackResponse packet with these slots present in it. (Or false with no slots, if rejected.)
- `legacy_set_item_slots.container_id` (`byte`)：
- `legacy_set_item_slots.slots` (`[]byte`)：
- `actions` (`[]InventoryAction`)：Actions is a list of actions that took place, that form the inventory transaction together. Each of these actions hold one slot in which one item was changed to another. In general, the combination of all of these actions results in a balanced inventory transaction. This should be checked to ensure that no items are cheated into the inventory.
- `actions.source_type` (`uint32`)：SourceType is the source type of the inventory action. It is one of the constants above.
- `actions.window_id` (`int32`)：WindowID is the ID of the window that the client has opened. The window ID is not set if the SourceType is InventoryActionSourceWorld.
- `actions.source_flags` (`uint32`)：SourceFlags is a combination of flags that is only set if the SourceType is InventoryActionSourceWorld.
- `actions.inventory_slot` (`uint32`)：InventorySlot is the slot in which the action took place. Each action only describes the change of item in a single slot.
- `actions.old_item` (`ItemInstance`)：OldItem is the item that was present in the slot before the inventory action. It should be checked by the server to ensure the inventories were not out of sync.
- `actions.old_item.stack_network_id` (`int32`)：StackNetworkID is the network ID of the item stack. If the stack is empty, 0 is always written for this field. If not, the field should be set to 1 if the server authoritative inventories are disabled in the StartGame packet, or to a unique stack ID if it is enabled.
- `actions.old_item.stack` (`ItemStack`)：Stack is the actual item stack of the item instance.
- `actions.old_item.stack.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `actions.old_item.stack.count` (`uint16`)：Count is the count of items that the item stack holds.
- `actions.old_item.stack.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `actions.old_item.stack.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `actions.old_item.stack.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `actions.old_item.stack.has_network_id` (`bool`)：HasNetworkID ...
- `actions.new_item` (`ItemInstance`)：NewItem is the new item that was put in the InventorySlot that the OldItem was in. It must be checked in combination with other inventory actions to ensure that the transaction is balanced.
- `actions.new_item.stack_network_id` (`int32`)：StackNetworkID is the network ID of the item stack. If the stack is empty, 0 is always written for this field. If not, the field should be set to 1 if the server authoritative inventories are disabled in the StartGame packet, or to a unique stack ID if it is enabled.
- `actions.new_item.stack` (`ItemStack`)：Stack is the actual item stack of the item instance.
- `actions.new_item.stack.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `actions.new_item.stack.count` (`uint16`)：Count is the count of items that the item stack holds.
- `actions.new_item.stack.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `actions.new_item.stack.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `actions.new_item.stack.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `actions.new_item.stack.has_network_id` (`bool`)：HasNetworkID ...
- `action_type` (`uint32`)：ActionType is the type of the UseItem inventory transaction. It is one of the action types found above, and specifies the way the player interacted with the block.
- `trigger_type` (`uint32`)：TriggerType is the type of the trigger that caused the inventory transaction. It is one of the trigger types found in the constants above. If TriggerType is TriggerTypePlayerInput, the transaction is from the initial input of the player. If it is TriggerTypeSimulationTick, the transaction is from a simulation tick when the player is holding down the input.
- `block_position` (`BlockPos`)：BlockPosition is the position of the block that was interacted with. This is only really a correct block position if ActionType is not UseItemActionClickAir.
- `block_face` (`int32`)：BlockFace is the face of the block that was interacted with. When clicking the block, it is the face clicked. When breaking the block, it is the face that was last being hit until the block broke.
- `hot_bar_slot` (`int32`)：HotBarSlot is the hot bar slot that the player was holding while clicking the block. It should be used to ensure that the hot bar slot and held item are correctly synchronised with the server.
- `held_item` (`ItemInstance`)：HeldItem is the item that was held to interact with the block. The server should check if this item is actually present in the HotBarSlot.
- `held_item.stack_network_id` (`int32`)：StackNetworkID is the network ID of the item stack. If the stack is empty, 0 is always written for this field. If not, the field should be set to 1 if the server authoritative inventories are disabled in the StartGame packet, or to a unique stack ID if it is enabled.
- `held_item.stack` (`ItemStack`)：Stack is the actual item stack of the item instance.
- `held_item.stack.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `held_item.stack.count` (`uint16`)：Count is the count of items that the item stack holds.
- `held_item.stack.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `held_item.stack.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `held_item.stack.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `held_item.stack.has_network_id` (`bool`)：HasNetworkID ...
- `position` (`mgl32.Vec3`)：Position is the position of the player at the time of interaction. For clicking a block, this is the position at that time, whereas for breaking the block it is the position at the time of breaking.
- `clicked_position` (`mgl32.Vec3`)：ClickedPosition is the position that was clicked relative to the block's base coordinate. It can be used to find out exactly where a player clicked the block.
- `block_runtime_id` (`uint32`)：BlockRuntimeID is the runtime ID of the block that was clicked. It may be used by the server to verify that the player's world client-side is synchronised with the server's.
- `client_prediction` (`uint32`)：ClientPrediction is the client's prediction on the output of the transaction. It is one of the client prediction found in the constants above.

### item_stack_request 嵌套字段

- `request_id` (`int32`)：RequestID is a unique ID for the request. This ID is used by the server to send a response for this specific request in the ItemStackResponse packet.
- `actions` (`[]StackRequestAction`)：Actions is a list of actions performed by the client. The actual type of the actions depends on which ID was present, and is one of the concrete types below.
- `filter_strings` (`[]string`)：FilterStrings is a list of filter strings involved in the request. This is typically filled with one string when an anvil or cartography is used.
- `filter_cause` (`int32`)：FilterCause represents the cause of any potential filtering. This is one of the constants above.

### block_actions 嵌套字段

- `action` (`int32`)：Action is the action to be performed, and is one of the constants listed above.
- `block_pos` (`BlockPos`)：BlockPos is the position of the block that was interacted with.
- `face` (`int32`)：Face is the face of the block that was interacted with.

## 145 · CreativeContent · 创造物品目录

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：服务端下发可用创造物品；列在目录不代表当前账号可取用。
- 本内核原始说明：CreativeContent is a packet sent by the server to set the creative inventory's content for a player. Introduced in 1.16, this packet replaces the previous method - sending an InventoryContent packet with creative inventory window ID. As of v1.21.60, this packet is no longer required to be sent as part of the login sequence however the client will crash if they try to open their creative inventory before receiving this packet. Every item must be part of a group, any items that are not part of a group will need to reference an "anonymous group" which has an empty name OR no icon. The order of Groups and Items is how the client will render items in the creative inventory compared to the previous, hard coded order.  Below is an example of defining 2 ungrouped items, 2 grouped items and then another 2 ungrouped items, all in the nature category.  CreativeContent{ Groups: []protocol.CreativeGroup{ {Category: 1}, // No name or icon, this is the "anonymous group" {Category: 1, Name: "itemGroup.name.planks", Icon: protocol.ItemStack{...}}, // A "planks" group {Category: 1}, // Another "anonymous group" }, Items: []protocol.CreativeItem{ {CreativeItemNetworkID: 0, Item: protocol.ItemStack{...}, GroupIndex: 0}, // Ungrouped before "planks" {CreativeItemNetworkID: 1, Item: protocol.ItemStack{...}, GroupIndex: 0}, // Ungrouped before "planks" {CreativeItemNetworkID: 2, Item: protocol.ItemStack{...}, GroupIndex: 1}, // Grouped under the "planks" group {CreativeItemNetworkID: 3, Item: protocol.ItemStack{...}, GroupIndex: 1}, // Grouped under the "planks" group {CreativeItemNetworkID: 4, Item: protocol.ItemStack{...}, GroupIndex: 2}, // Ungrouped after "planks" {CreativeItemNetworkID: 5, Item: protocol.ItemStack{...}, GroupIndex: 2}, // Ungrouped after "planks" } }

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `groups` | `[]protocol.CreativeGroup` | Groups is a list of the groups that should be added to the creative inventory. |
| `items` | `[]protocol.CreativeItem` | Items is a list of the items that should be added to the creative inventory. |

### groups 嵌套字段

- `category` (`int32`)：Category is the category the group falls under. It is one of the constants above.
- `name` (`string`)：Name is the locale name of the group, i.e. "itemGroup.name.planks".
- `icon` (`ItemStack`)：Icon is the item that represents the group in the creative inventory.
- `icon.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `icon.count` (`uint16`)：Count is the count of items that the item stack holds.
- `icon.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `icon.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `icon.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `icon.has_network_id` (`bool`)：HasNetworkID ...

### items 嵌套字段

- `creative_item_network_id` (`uint32`)：CreativeItemNetworkID is a unique ID for the creative item. It has to be unique for each creative item sent to the client. An incrementing ID per creative item does the job.
- `item` (`ItemStack`)：Item is the item that should be added to the creative inventory.
- `item.block_runtime_id` (`int32`)：BlockRuntimeID ...
- `item.count` (`uint16`)：Count is the count of items that the item stack holds.
- `item.nbtdata` (`map[string]any`)：NBTData is a map that is serialised to its NBT representation when sent in a packet.
- `item.can_be_placed_on` (`[]string`)：CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item that can be placed, can be placed on top of.
- `item.can_break` (`[]string`)：CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
- `item.has_network_id` (`bool`)：HasNetworkID ...
- `group_index` (`uint32`)：GroupIndex is the index of the group that the item should be placed in. It is the index of the group in the CreativeContent packet previously sent to the client.

## 146 · PlayerEnchantOptions · 玩家附魔选项

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：PlayerEnchantOptions is sent by the server to update the enchantment options displayed when the user opens the enchantment table and puts an item in. This packet was added in 1.16 and allows the server to decide on the enchantments that can be selected by the player. The PlayerEnchantOptions packet should be sent once for every slot update of the enchantment table. The vanilla server sends an empty PlayerEnchantOptions packet when the player opens the enchantment table (air is present in the enchantment table slot) and sends the packet with actual enchantments in it when items are put in that can have enchantments.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `options` | `[]protocol.EnchantmentOption` | Options is a list of possible enchantment options for the item that was put into the enchantment table. |

### options 嵌套字段

- `cost` (`uint32`)：Cost is the cost of the option. This is the amount of XP levels required to select this enchantment option.
- `enchantments` (`ItemEnchantments`)：Enchantments holds the enchantments that will be applied to the item when this option is clicked.
- `enchantments.slot` (`int32`)：Slot is either 0, 1 or 2. Its exact usage is not clear.
- `enchantments.enchantments` (`[3][]EnchantmentInstance`)：Enchantments is an array of 3 slices of enchantment instances. Each array represents enchantments that will be added to the item with a different activation type. The arrays in which enchantments are sent by the vanilla server are as follows: slice 1 { protection, fire protection, feather falling, blast protection, projectile protection, thorns, respiration, depth strider, aqua affinity, frost walker, soul speed } slice 2 { sharpness, smite, bane of arthropods, fire aspect, looting, silk touch, unbreaking, fortune, flame, luck of the sea, impaling } slice 3 { knockback, efficiency, power, punch, infinity, lure, mending, curse of binding, curse of vanishing, riptide, loyalty, channeling, multishot, piercing, quick charge } The first slice holds armour enchantments, the differences between the slice 2 and slice 3 are more vaguely defined.
- `name` (`string`)：Name is a name that will be translated to the 'Standard Galactic Alphabet' (https://minecraft.wiki/w/Enchanting_Table#Standard_Galactic_Alphabet) client-side. The names generally have no meaning, such as: 'animal imbue range galvanize ' 'bless inside creature shrink ' 'elder free of inside '
- `recipe_network_id` (`uint32`)：RecipeNetworkID is a unique network ID for this enchantment option. When enchanting, the client will submit this network ID in a ItemStackRequest packet with the CraftRecipe action, so that the server knows which enchantment was selected. Note that this ID should still be unique with other actual recipes. It's recommended to start counting for enchantment network IDs from the counter used for producing network IDs for the normal recipes.

## 147 · ItemStackRequest · 现代物品栈请求

- 分组：物品与容器
- 方向：客户端→服务器
- 用法与边界：移动、交换、丢弃、合成等操作，由服务器校验，不能伪造得到任意物品。
- 本内核原始说明：ItemStackRequest represents a single request present in an ItemStackRequest packet sent by the client to change an item in an inventory. Item stack requests are either approved or rejected by the server using the ItemStackResponse packet.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `requests` | `[]protocol.ItemStackRequest` |  |

### requests 嵌套字段

- `request_id` (`int32`)：RequestID is a unique ID for the request. This ID is used by the server to send a response for this specific request in the ItemStackResponse packet.
- `actions` (`[]StackRequestAction`)：Actions is a list of actions performed by the client. The actual type of the actions depends on which ID was present, and is one of the concrete types below.
- `filter_strings` (`[]string`)：FilterStrings is a list of filter strings involved in the request. This is typically filled with one string when an anvil or cartography is used.
- `filter_cause` (`int32`)：FilterCause represents the cause of any potential filtering. This is one of the constants above.

## 148 · ItemStackResponse · 物品栈请求响应

- 分组：物品与容器
- 方向：服务器→本机客户端
- 用法与边界：匹配请求ID看成功失败与槽位，不将请求发送当事务成功。
- 本内核原始说明：ItemStackResponse is a response to an individual ItemStackRequest.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `responses` | `[]protocol.ItemStackResponse` |  |

### responses 嵌套字段

- `status` (`uint8`)：Status specifies if the request with the RequestID below was successful. If this is the case, the ContainerInfo below will have information on what slots ended up changing. If not, the container info will be empty. A non-0 status means an error occurred and will result in the action being reverted.
- `request_id` (`int32`)：RequestID is the unique ID of the request that this response is in reaction to. If rejected, the client will undo the actions from the request with this ID.
- `container_info` (`[]StackResponseContainerInfo`)：ContainerInfo holds information on the containers that had their contents changed as a result of the request.
- `container_info.container` (`FullContainerName`)：Container is the FullContainerName that describes the container that the slots that follow are in. For the main inventory, the ContainerID seems to be 0x1b. Fur the cursor, this value seems to be 0x3a. For the crafting grid, this value seems to be 0x0d.
- `container_info.container.container_id` (`byte`)：ContainerID is the ID of the container that the slot was in.
- `container_info.container.dynamic_container_id` (`Optional[uint32]`)：DynamicContainerID is the ID of the container if it is dynamic. If the container is not dynamic, this field should be left empty. A non-optional value of 0 is assumed to be non-empty.
- `container_info.slot_info` (`[]StackResponseSlotInfo`)：SlotInfo holds information on what item stack should be present in specific slots in the container.
- `container_info.slot_info.count` (`byte`)：Count is the total count of the item stack. This count will be shown client-side after the response is sent to the client.
- `container_info.slot_info.stack_network_id` (`int32`)：StackNetworkID is the network ID of the new stack at a specific slot.
- `container_info.slot_info.custom_name` (`string`)：CustomName is the custom name of the item stack. It is used in relation to text filtering.
- `container_info.slot_info.filtered_custom_name` (`string`)：FilteredCustomName is a filtered version of CustomName with all the profanity removed. The client will use this over CustomName if this field is not empty and they have the "Filter Profanity" setting enabled.
- `container_info.slot_info.durability_correction` (`int32`)：DurabilityCorrection is the current durability of the item stack. This durability will be shown client-side after the response is sent to the client.

## 149 · PlayerArmourDamage · 玩家护甲伤害

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：PlayerArmourDamage is sent by the server to damage the armour of a player. It is a very efficient packet, but generally it's much easier to just send a slot update for the damaged armour.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `bitset` | `uint8` | Bitset holds a bitset of 4 bits that indicate which pieces of armour need to have damage dealt to them. The first bit, when toggled, is for a helmet, the second for the chestplate, the third for the leggings and the fourth for boots. |
| `helmet_damage` | `int32` | HelmetDamage is the amount of damage that should be dealt to the helmet. |
| `chestplate_damage` | `int32` | ChestplateDamage is the amount of damage that should be dealt to the chestplate. |
| `leggings_damage` | `int32` | LeggingsDamage is the amount of damage that should be dealt to the leggings. |
| `boots_damage` | `int32` | BootsDamage is the amount of damage that should be dealt to the boots. |
| `body_damage` | `int32` | BodyDamage is the amount of damage that should be dealt to the body. |

## 150 · CodeBuilder · 代码构建

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：CodeBuilder is an Education Edition packet sent by the server to the client to open the URL to a Code Builder (websocket) server.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `url` | `string` | URL is the url to the Code Builder (websocket) server. |
| `should_open_code_builder` | `bool` | ShouldOpenCodeBuilder specifies if the client should automatically open the Code Builder app. If set to true, the client will attempt to use the Code Builder app to connect to and interface with the server running at the URL above. |

## 151 · UpdatePlayerGameType · 玩家游戏模式变更

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：同步特定实体的游戏模式。
- 本内核原始说明：UpdatePlayerGameType is sent by the server to change the game mode of a player. It is functionally identical to the SetPlayerGameType packet.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `game_type` | `int32` | GameType is the new game type of the player. It is one of the constants that can be found in set_player_game_type.go. Some of these game types require additional flags to be set in an UpdateAbilities packet for the game mode to obtain its full functionality. |
| `player_unique_id` | `int64` | PlayerUniqueID is the entity unique ID of the player that should have its game mode updated. If this packet is sent to other clients with the player unique ID of another player, nothing happens. |
| `tick` | `uint64` | 输入/同步的游戏刻编号。 Tick is the server tick at which the packet was sent. It is used in relation to CorrectPlayerMovePrediction. |

## 152 · EmoteList · 表情动作目录

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：实体可用表情动作ID列表，须客户端资源支持。
- 本内核原始说明：EmoteList is sent by the client every time it joins the server and when it equips new emotes. It may be used by the server to find out which emotes the client has available. If the player has no emotes equipped, this packet is not sent. Under certain circumstances, this packet is also sent from the server to the client, but I was unable to find when this is done.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `player_runtime_id` | `uint64` | PlayerRuntimeID is the runtime ID of the player that owns the emote pieces below. If sent by the client, this player runtime ID is always that of the player itself. |
| `emote_pieces` | `[]uuid.UUID` | EmotePieces is a list of emote pieces that the player with the runtime ID above has. |

## 153 · PositionTrackingDBServerBroadcast · 位置追踪数据库广播

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：位置追踪标识和数据；不是查询全部玩家坐标的万能入口。
- 本内核原始说明：PositionTrackingDBServerBroadcast is sent by the server in response to the PositionTrackingDBClientRequest packet. This packet is, as of 1.16, currently only used for lodestones. The server maintains a database with tracking IDs and their position and dimension. The client will request these tracking IDs, (NBT tag set on the lodestone compass with the tracking ID?) and the server will respond with the status of those tracking IDs. What is actually done with the data sent depends on what the client chooses to do with it. For the lodestone compass, it is used to make the compass point towards lodestones and to make it spin if the lodestone at a position is no longer there.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `broadcast_action` | `uint8` | BroadcastAction specifies the status of the position tracking DB response. It is one of the constants above, specifying the result of the request with the ID below. The Update action is sent for setting the position of a lodestone compass, the Destroy and NotFound to indicate that there is not (no longer) a lodestone at that position. |
| `tracking_id` | `int32` | TrackingID is the ID of the PositionTrackingDBClientRequest packet that this packet was in response to. The tracking ID is also present as the 'id' field in the SerialisedData field. |
| `payload` | `map[string]interface {}` | Payload is a network little endian compound tag holding the data retrieved from the position tracking DB. An example data structure sent if BroadcastAction is of the type Update: TAG_Compound({ 'version': TAG_Byte(0x01), 'dim': TAG_Int(0), 'id': TAG_String(0x00000001), 'pos': TAG_List<TAG_Int>({ -299, 86, 74, }), 'status': TAG_Byte(0x00), // 0x00 for updating, 0x02 for not found/block destroyed. }) |

## 154 · PositionTrackingDBClientRequest · 位置追踪数据库请求

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：客户端请求追踪数据，依赖有效ID及服务器支持。
- 本内核原始说明：PositionTrackingDBClientRequest is a packet sent by the client to request the position and dimension of a 'tracking ID'. These IDs are tracked in a database by the server. In 1.16, this is used for lodestones. The client will send this request to find the position a lodestone compass needs to point to. If found, it will point to the lodestone. If not, it will start spinning around. A PositionTrackingDBServerBroadcast packet should be sent in response to this packet.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `request_action` | `uint8` | RequestAction is the action that should be performed upon the receiving of the packet. It is one of the constants found above. |
| `tracking_id` | `int32` | TrackingID is a unique ID used to identify the request. The server responds with a PositionTrackingDBServerBroadcast packet holding the same ID, so that the client can find out what that packet was in response to. |

## 155 · DebugInfo · 调试信息

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：DebugInfo is a packet sent by the server to the client. It does not seem to do anything when sent to the normal client in 1.16.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `player_unique_id` | `int64` | PlayerUniqueID is the unique ID of the player that the packet is sent to. |
| `data` | `[]uint8` | Data is the debug data. |

## 156 · PacketViolationWarning · 协议违规告警

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：服务器报告包违规，结合包ID和告警说明修复发包。
- 本内核原始说明：PacketViolationWarning is sent by the client when it receives an invalid packet from the server. It holds some information on the error that occurred. noinspection GoNameStartsWithPackageName

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `type` | `int32` | Type is the type of violation. It is one of the constants above. |
| `severity` | `int32` | Severity specifies the severity of the packet violation. The action the client takes after this violation depends on the severity sent. |
| `packet_id` | `int32` | PacketID is the ID of the invalid packet that was received. |
| `violation_context` | `string` | ViolationContext holds a description on the violation of the packet. |

## 157 · MotionPredictionHints · 运动预测提示

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：服务端提供运动预测参数，用于客户端同步。
- 本内核原始说明：MotionPredictionHints is sent by the server to the client. There is a predictive movement component for entities. This packet fills the "history" of that component and entity movement is computed based on the points. Vanilla sends this packet instead of the SetActorMotion packet when 'spatial optimisations' are enabled.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the entity whose velocity is sent to the client. |
| `velocity` | `mgl32.Vec3` | 协议速度向量，单位需结合游戏刻核对。 Velocity is the server-calculated velocity of the entity at the point of sending the packet. |
| `on_ground` | `bool` | OnGround specifies if the server currently thinks the entity is on the ground. |

## 158 · AnimateEntity · 动画实体

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：AnimateEntity is sent by the server to animate an entity client-side. It may be used to play a single animation, or to activate a controller which can start a sequence of animations based on different conditions specified in an animation controller. Much of the documentation of this packet can be found at https://learn.microsoft.com/en-us/minecraft/creator/reference/content/animationsreference

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `animation` | `string` | Animation is the name of a single animation to start playing. |
| `next_state` | `string` | NextState is the first state to start with. These states are declared in animation controllers (which, in themselves, are animations too). These states in turn may have animations and transitions to move to a next state. |
| `stop_condition` | `string` | StopCondition is a MoLang expression that specifies when the animation should be stopped. |
| `stop_condition_version` | `int32` | StopConditionVersion is the MoLang stop condition version. |
| `controller` | `string` | Controller is the animation controller that is used to manage animations. These controllers decide when to play which animation. |
| `blend_out_time` | `float32` | BlendOutTime does not currently seem to be used. |
| `entity_runtime_ids` | `[]uint64` | EntityRuntimeIDs is list of runtime IDs of entities that the animation should be applied to. |

## 159 · CameraShake · 摄像机震动

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：CameraShake is sent by the server to make the camera shake client-side. This feature was added for map- making partners.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `intensity` | `float32` | Intensity is the intensity of the shaking. The client limits this value to 4, so anything higher may not work. |
| `duration` | `float32` | Duration is the number of seconds the camera will shake for. |
| `type` | `uint8` | Type is the type of shake, and is one of the constants listed above. The different type affects how the shake looks in game. |
| `action` | `uint8` | Action is the action to be performed, and is one of the constants listed above. Currently the different actions will either add or stop shaking the client. |

## 160 · PlayerFog · 玩家雾

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：PlayerFog is sent by the server to render the different fogs in the Stack. The types of fog are controlled by resource packs to change how they are rendered, and the ability to create custom fog.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `stack` | `[]string` | Stack is a list of fog identifiers to be sent to the client. Examples of fog identifiers are "minecraft:fog_ocean" and "minecraft:fog_hell". |

## 161 · CorrectPlayerMovePrediction · 校正玩家移动预测

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：CorrectPlayerMovePrediction is sent by the server if and only if StartGame.ServerAuthoritativeMovementMode is set to AuthoritativeMovementModeServerWithRewind. The packet is used to correct movement at a specific point in time.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `prediction_type` | `uint8` | PredictionType is the type of prediction that was corrected. It is one of the constants above. |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position that the player is supposed to be at the tick written in the field below. The client will change its current position based on movement after that tick starting from the Position. |
| `delta` | `mgl32.Vec3` | Delta is the change in position compared to what the client sent as its position at that specific tick. |
| `rotation` | `mgl32.Vec2` | Rotation is the rotation of the player at the tick written in the field below. It is only included if PredictionType is PredictionTypeVehicle. |
| `vehicle_angular_velocity` | `protocol.Optional[float32]` | VehicleAngularVelocity is the angular velocity of the vehicle that the rider is riding. |
| `on_ground` | `bool` | OnGround specifies if the player was on the ground at the time of the tick below. |
| `tick` | `uint64` | 输入/同步的游戏刻编号。 Tick is the tick of the movement which was corrected by this packet. |

## 162 · ItemRegistry · 物品注册表

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ItemRegistry is sent by the server to send the client a list of available items and attach client-side components to a custom item. This packet was formerly known as the ItemComponent packet before 1.21.60, which did not include item definitions but only the components.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `items` | `[]protocol.ItemEntry` | Items is a list of all items with their legacy IDs which are available in the game. Failing to send any of the items that are in the game will crash mobile clients. Any custom components are also attached to the items in this list. |

### items 嵌套字段

- `name` (`string`)：Name if the name of the item, which is a name like 'minecraft:stick'.
- `runtime_id` (`int16`)：RuntimeID is the ID that is used to identify the item over network. After sending all items in the StartGame packet, items will then be identified using these numerical IDs.
- `component_based` (`bool`)：ComponentBased specifies if the item was created using components, meaning the item is a custom item.
- `version` (`int32`)：Version is the version of the item entry which is used by the client to determine how to handle the item entry. It is one of the constants above.
- `data` (`map[string]any`)：Data is a map containing the components and properties of the item, if the item is component based.

## 163 · FilterText · 文本过滤

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：过滤文本请求/响应，不能默认当作全服聊天记录。
- 本内核原始说明：FilterText is sent by the both the client and the server. The client sends the packet to the server to allow the server to filter the text server-side. The server then responds with the same packet and the safer version of the text.  Deprecated: This packet was deprecated in unknown.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `text` | `string` | Text is either the text from the client or the safer version of the text sent by the server. |
| `from_server` | `bool` | FromServer indicates if the packet was sent by the server or not. |

## 164 · ClientBoundDebugRenderer · 客户端定向调试渲染器

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ClientBoundDebugRenderer is sent by the server to spawn an outlined cube on client-side.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `type` | `uint32` | Type is the type of action. It is one of the constants above. |
| `text` | `string` | Text is the text that is displayed above the debug. |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position to spawn the debug on. |
| `red` | `float32` | Red is the red value from the RGBA colour rendered on the debug. This value is in the range 0-1. |
| `green` | `float32` | Green is the green value from the RGBA colour rendered on the debug. This value is in the range 0-1. |
| `blue` | `float32` | Blue is the blue value from the RGBA colour rendered on the debug. This value is in the range 0-1. |
| `alpha` | `float32` | Alpha is the alpha value from the RGBA colour rendered on the debug. This value is in the range 0-1. |
| `duration` | `uint64` | Duration is how long the debug will last in the world for. It is measured in milliseconds. |

## 165 · SyncActorProperty · 同步实体属性

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：SyncActorProperty is an alternative to synced actor data.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `property_data` | `map[string]interface {}` | PropertyData ... |

## 166 · AddVolumeEntity · 添加区域实体

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：AddVolumeEntity sends a volume entity's definition and metadata from server to client.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the volume. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `entity_metadata` | `map[string]interface {}` | EntityMetadata is a map of entity metadata, which includes flags and data properties that alter in particular the way the volume functions or looks. |
| `encoding_identifier` | `string` | EncodingIdentifier is the unique identifier for the volume. It must be of the form 'namespace:name', where namespace cannot be 'minecraft'. |
| `instance_identifier` | `string` | InstanceIdentifier is the identifier of a fog definition. |
| `bounds` | `[2]protocol.BlockPos` | Bounds represent the volume's bounds. The first value is the minimum bounds, and the second value is the maximum bounds. |
| `dimension` | `int32` | Dimension is the dimension in which the volume exists. |
| `engine_version` | `string` | EngineVersion is the engine version the entity is using, for example, '1.17.0'. |

## 167 · RemoveVolumeEntity · 移除区域实体

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：RemoveVolumeEntity indicates a volume entity to be removed from server to client.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID .. |
| `dimension` | `int32` | Dimension ... |

## 168 · SimulationType · 模拟模式

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：同步游戏模拟模式，具体作用以当前服务端为准。
- 本内核原始说明：SimulationType is an in-progress packet. We currently do not know the use case.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `simulation_type` | `uint8` | SimulationType is the simulation type selected. |

## 169 · NPCDialogue · 非玩家角色对话

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：NPC对话内容与场景，用于交互菜单解析。
- 本内核原始说明：NPCDialogue is a packet that allows the client to display dialog boxes for interacting with NPCs.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_unique_id` | `uint64` | 实体唯一ID，与运行ID不同。 EntityUniqueID is the unique ID of the NPC being requested. |
| `action_type` | `int32` | 动作枚举，具体编号使用本内核定义。 ActionType is the type of action for the packet. |
| `dialogue` | `string` | Dialogue is the text that the client should see. |
| `scene_name` | `string` | SceneName is the identifier of the scene. If this is left empty, the client will use the last scene sent to it. https://docs.microsoft.com/en-us/minecraft/creator/documents/npcdialogue. |
| `npcname` | `string` | NPCName is the name of the NPC to be displayed to the client. |
| `action_json` | `string` | ActionJSON is the JSON string of the buttons/actions the server can perform. |

## 170 · EducationResourceURI · 教育资源地址

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：教育版资源URI信息，与普通插件API调用无关。
- 本内核原始说明：EducationResourceURI is a packet that transmits education resource settings to all clients.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `resource` | `protocol.EducationSharedResourceURI` | Resource is the resource that is being referenced. |

### resource 嵌套字段

- `button_name` (`string`)：ButtonName is the button name of the resource URI.
- `link_uri` (`string`)：LinkURI is the link URI for the resource URI.

## 171 · CreatePhoto · 创建照片

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：教育/相机照片功能请求，需对应环境支持。
- 本内核原始说明：CreatePhoto is a packet that allows players to export photos from their portfolios into items in their inventory. This packet only works on the Education Edition version of Minecraft.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_unique_id` | `int64` | 实体唯一ID，与运行ID不同。 EntityUniqueID is the unique ID of the entity. |
| `photo_name` | `string` | PhotoName is the name of the photo. |
| `item_name` | `string` | ItemName is the name of the photo as an item. |

## 172 · UpdateSubChunkBlocks · 子区块方块更新

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：普通层和额外层的方块变更列表；只表示服务器同步的更新。
- 本内核原始说明：UpdateSubChunkBlocks is essentially just UpdateBlock packet, however for a set of blocks in a sub-chunk.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `position` | `protocol.SubChunkPos` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position of the sub-chunk being referred to. |
| `blocks` | `[]protocol.BlockChangeEntry` | Blocks contains each updated block change entry. |
| `extra` | `[]protocol.BlockChangeEntry` | Extra contains each updated block change entry for the second layer, usually for waterlogged blocks. |

### blocks 嵌套字段

- `block_runtime_id` (`uint32`)：BlockRuntimeID is the runtime ID of the block.
- `flags` (`uint32`)：Flags is a combination of flags that specify the way the block is updated client-side.
- `synced_update_entity_unique_id` (`uint64`)：SyncedUpdateEntityUniqueID  is the unique ID of the falling block entity that the block transitions to or that the entity transitions from if the block change entry is synced.
- `synced_update_type` (`uint32`)：SyncedUpdateType is the type of the transition that happened. It is either BlockToEntityTransition, when a block placed becomes a falling entity, or EntityToBlockTransition, when a falling entity hits the ground and becomes a solid block again.

### extra 嵌套字段

- `block_runtime_id` (`uint32`)：BlockRuntimeID is the runtime ID of the block.
- `flags` (`uint32`)：Flags is a combination of flags that specify the way the block is updated client-side.
- `synced_update_entity_unique_id` (`uint64`)：SyncedUpdateEntityUniqueID  is the unique ID of the falling block entity that the block transitions to or that the entity transitions from if the block change entry is synced.
- `synced_update_type` (`uint32`)：SyncedUpdateType is the type of the transition that happened. It is either BlockToEntityTransition, when a block placed becomes a falling entity, or EntityToBlockTransition, when a falling entity hits the ground and becomes a solid block again.

## 173 · PhotoInfoRequest · 照片信息请求

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：PhotoInfoRequest is sent by the client to request photo information from the server.  Deprecated: This packet was deprecated in 1.19.80.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `photo_id` | `int64` | PhotoID is the ID of the photo. |

## 174 · SubChunk · 子区块响应

- 分组：世界与结构
- 方向：服务器→客户端
- 用法与边界：对应SubChunkRequest，需按当前维度与调色板解析；不能把原始payload_hex当方块名。
- 本内核原始说明：SubChunk sends data about multiple sub-chunks around a center point.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `cache_enabled` | `bool` | CacheEnabled is whether the sub-chunk caching is enabled or not. |
| `dimension` | `int32` | Dimension is the dimension the sub-chunks are in. |
| `position` | `protocol.SubChunkPos` | 位置坐标，维度与有效性按本包上下文确定。 Position is an absolute sub-chunk center point that every SubChunkRequest uses as a reference. |
| `sub_chunk_entries` | `[]protocol.SubChunkEntry` | SubChunkEntries contains sub-chunk entries relative to the center point. |
| `block_entities` | `[]uint8` | BlockEntities contains the trailing NBT block entity data after all sub-chunk entries. This data is only present when the server sends block entities in the SubChunk response. |

### sub_chunk_entries 嵌套字段

- `offset` (`SubChunkOffset`)：Offset contains the offset between the sub-chunk position and the center position.
- `result` (`byte`)：Result is always one of the constants defined in the SubChunkResult constants.
- `raw_payload` (`[]byte`)：RawPayload contains the serialized sub-chunk data.
- `height_map_type` (`byte`)：HeightMapType is always one of the constants defined in the HeightMapData constants.
- `height_map_data` (`[]int8`)：HeightMapData is the data for the height map.
- `render_height_map_type` (`byte`)：RenderHeightMapType is always one of the constants defined in the HeightMapData constants.
- `render_height_map_data` (`[]int8`)：RenderHeightMapData is the data for the render height map.
- `blob_hash` (`uint64`)：BlobHash is the hash of the blob.

## 175 · SubChunkRequest · 子区块请求

- 分组：世界与结构
- 方向：客户端→服务器
- 用法与边界：请求范围受实际连接、视距和服务端限制；发送后必须等待响应。
- 本内核原始说明：SubChunkRequest requests specific sub-chunks from the server using a center point.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `dimension` | `int32` | Dimension is the dimension of the sub-chunk. |
| `position` | `protocol.SubChunkPos` | 位置坐标，维度与有效性按本包上下文确定。 Position is an absolute sub-chunk center point used as a base point for all sub-chunks requested. The X and Z coordinates represent the chunk coordinates, while the Y coordinate is the absolute sub-chunk index. |
| `offsets` | `[]protocol.SubChunkOffset` | Offsets contains all requested offsets around the center point. |

## 176 · ClientStartItemCooldown · 物品冷却启动

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：服务端同步本机物品类别与冷却刻数。
- 本内核原始说明：ClientStartItemCooldown is sent by the server to the client to initiate a cooldown on an item.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `category` | `string` | Category is the category of the item to start the cooldown on. |
| `duration` | `int32` | Duration is the duration of ticks the cooldown should last. |

## 177 · ScriptMessage · 脚本消息

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：脚本消息通道的ID与内容，需要服务端脚本约定，不能任意调用全部服务端函数。
- 本内核原始说明：ScriptMessage is used to communicate custom messages from the client to the server, or from the server to the client. While the name may suggest this packet is used for the discontinued scripting API, it is likely instead for the GameTest framework.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `identifier` | `string` | Identifier is the identifier of the message, used by either party to identify the message data sent. |
| `data` | `[]uint8` | Data contains the data of the message. |

## 178 · CodeBuilderSource · 代码构建来源

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：CodeBuilderSource is an Education Edition packet sent by the client to the server to run an operation with a code builder.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `operation` | `uint8` | Operation is used to distinguish the operation performed. It is always one of the constants listed above. |
| `category` | `uint8` | Category is used to distinguish the category of the operation performed. It is always one of the constants listed above. |
| `code_status` | `uint8` | CodeStatus is the status of the code builder. It is always one of the constants listed above. |

## 179 · TickingAreasLoadStatus · 常加载区域状态

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：服务器通知常加载区域是否已预加载。
- 本内核原始说明：TickingAreasLoadStatus is sent by the server to the client to notify the client of a ticking area's loading status.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `preload` | `bool` | Preload is true if the server is waiting for the area's preload. |

## 180 · DimensionData · 维度数据

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：DimensionData is a packet sent from the server to the client containing information about data-driven dimensions that the server may have registered. This packet does not seem to be sent by default, rather only being sent when any data-driven dimensions are registered.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `definitions` | `[]protocol.DimensionDefinition` | Definitions contain a list of data-driven dimension definitions registered on the server. |

### definitions 嵌套字段

- `name` (`string`)：Name specifies the name of the dimension.
- `range` (`[2]int32`)：Range is the height range of the dimension, where the first value is the minimum and the second is the maximum.
- `generator` (`int32`)：Generator is the variant of generator that exists in the provided dimension. These can be one of the constants defined above. If this is set to GeneratorLegacy, the legacy horizontal world limits will be enforced.

## 181 · AgentAction · 教育代理动作

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：教育代理动作状态或请求，不能代替普通玩家挖掘权限。
- 本内核原始说明：AgentAction is an Education Edition packet sent from the server to the client to return a response to a previously requested action.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `identifier` | `string` | Identifier is a JSON identifier referenced in the initial action. |
| `action` | `int32` | Action represents the action type that was requested. It is one of the constants defined above. |
| `response` | `[]uint8` | Response is a JSON string containing the response to the action. |

## 182 · ChangeMobProperty · 生物属性变更

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：实体属性变更，按内核实际字段读取。
- 本内核原始说明：ChangeMobProperty is a packet sent from the server to the client to change one of the properties of a mob client-side.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_unique_id` | `int64` | 实体唯一ID，与运行ID不同。  |
| `property` | `string` | EntityUniqueID uint64 Property is the name of the property being updated. |
| `bool_value` | `bool` | BoolValue is set if the property value is a bool type. If the type is not a bool, this field is ignored. |
| `string_value` | `string` | StringValue is set if the property value is a string type. If the type is not a string, this field is ignored. |
| `int_value` | `int32` | IntValue is set if the property value is an int type. If the type is not an int, this field is ignored. |
| `float_value` | `float32` | FloatValue is set if the property value is a float type. If the type is not a float, this field is ignored. |

## 183 · LessonProgress · 课程进度

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：LessonProgress is a packet sent by the server to the client to inform the client of updated progress on a lesson. This packet only functions on the Minecraft: Education Edition version of the game.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `identifier` | `string` | Identifier is the identifier of the lesson that is being progressed. |
| `action` | `uint8` | Action is the action the client should perform to show progress. This is one of the constants defined above. |
| `score` | `int32` | Score is the score the client should use when displaying the progress. |

## 184 · RequestAbility · 请求能力

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：RequestAbility is a packet sent by the client to the server to request permission for a specific ability from the server. These abilities are defined above.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `ability` | `int32` | Ability is the ability that the client is requesting. This is one of the constants defined in the protocol/ability.go file. |
| `value` | `interface {}` | Value represents the value of the ability. This can either be a boolean or a float32, otherwise the writer/reader will panic. |

## 185 · RequestPermissions · 请求调整权限

- 分组：权限与会话
- 方向：客户端→服务器
- 用法与边界：需要服务端允许的OP/Host权限，发包不绕过检查。
- 本内核原始说明：RequestPermissions is a packet sent from the client to the server to request permissions that the client does not currently have. It can only be sent by operators and host in vanilla Minecraft.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_unique_id` | `int64` | 实体唯一ID，与运行ID不同。 EntityUniqueID is the unique ID of the player. The unique ID is unique for the entire world and is often used in packets. Most servers send an EntityUniqueID equal to the EntityRuntimeID. |
| `permission_level` | `uint8` | PermissionLevel is the current permission level of the player. This is one of the constants that may be found in the AdventureSettings packet. |
| `requested_permissions` | `uint16` | RequestedPermissions contains the requested permission flags. |

## 186 · ToastRequest · 通知请求

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ToastRequest is a packet sent from the server to the client to display a toast to the top of the screen. These toasts are the same as the ones seen when, for example, loading a new resource pack or obtaining an achievement.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `title` | `string` | Title is the title of the toast. |
| `message` | `string` | Message is the message that the toast may contain alongside the title. |

## 187 · UpdateAbilities · 实体能力与权限同步

- 分组：权限与会话
- 方向：服务器→客户端
- 用法与边界：真实能力位和权限等级，可查当前授予能力，不等于可以自行授予。
- 本内核原始说明：UpdateAbilities is a packet sent from the server to the client to update the abilities of the player. It, along with the UpdateAdventureSettings packet, are replacements of the AdventureSettings packet since v1.19.10.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `ability_data` | `protocol.AbilityData` | AbilityData represents various data about the abilities of a player, such as ability layers or permissions. |

### ability_data 嵌套字段

- `entity_unique_id` (`int64`)：EntityUniqueID is a unique identifier of the player. It appears it is not required to fill this field out with a correct value. Simply writing 0 seems to work.
- `player_permissions` (`byte`)：PlayerPermissions is the permission level of the player as it shows up in the player list built up using the PlayerList packet.
- `command_permissions` (`byte`)：CommandPermissions is a set of permissions that specify what commands a player is allowed to execute.
- `layers` (`[]AbilityLayer`)：Layers contains all ability layers and their potential values. This should at least have one entry, being the base layer.
- `layers.type` (`uint16`)：Type represents the type of the layer. This is one of the AbilityLayerType constants defined above.
- `layers.abilities` (`uint32`)：Abilities is a set of abilities that are enabled for the layer. This is one of the Ability constants defined above.
- `layers.values` (`uint32`)：Values is a set of values that are associated with the enabled abilities, representing the values of the abilities.
- `layers.fly_speed` (`float32`)：FlySpeed is the default horizontal fly speed of the layer.
- `layers.vertical_fly_speed` (`float32`)：VerticalFlySpeed is the default vertical fly speed of the layer.
- `layers.walk_speed` (`float32`)：WalkSpeed is the default walk speed of the layer.

## 188 · UpdateAdventureSettings · 冒险能力更新

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：更新玩家能力设置，服务器仍决定权限。
- 本内核原始说明：UpdateAdventureSettings is a packet sent from the server to the client to update the adventure settings of the player. It, along with the UpdateAbilities packet, are replacements of the AdventureSettings packet since v1.19.10.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `no_pv_m` | `bool` | NoPvM is a boolean indicating whether the player is allowed to fight mobs or not. |
| `no_mv_p` | `bool` | NoMvP is a boolean indicating whether mobs are allowed to fight the player or not. It is unclear why this is sent to the client. |
| `immutable_world` | `bool` | ImmutableWorld is a boolean indicating whether the player is allowed to modify the world or not. |
| `show_name_tags` | `bool` | ShowNameTags is a boolean indicating whether player name tags are shown or not. |
| `auto_jump` | `bool` | AutoJump is a boolean indicating whether the player is allowed to jump automatically or not. |

## 189 · DeathInfo · 本机死亡信息

- 分组：属性与状态
- 方向：服务器→本机客户端
- 用法与边界：本机死亡提示或原因参数；不是全服每次死亡记录；翻译key与parameters需按实际包解释。
- 本内核原始说明：DeathInfo is a packet sent from the server to the client expected to be sent when a player dies. It contains messages related to the player's death, which are shown on the death screen as of v1.19.10.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `cause` | `string` | Cause is the cause of the player's death, such as "suffocation" or "suicide". |
| `messages` | `[]string` | Messages is a list of death messages to be shown on the death screen. |

## 190 · EditorNetwork · 编辑器网络

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：编辑器专用负载，不是通用游戏命令批处理协议。
- 本内核原始说明：EditorNetwork is a packet sent from the server to the client and vise-versa to communicate editor-mode related information. It carries a single compound tag containing the relevant information.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `route_to_manager` | `bool` | RouteToManager ... |
| `payload` | `map[string]interface {}` | Payload is a network little endian compound tag holding data relevant to the editor. |

## 191 · FeatureRegistry · 特征注册表

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：FeatureRegistry is a packet used to notify the client about the world generation features the server is currently using. This is used in combination with the client-side world generation system introduced in v1.19.20, allowing the client to completely generate the chunks of the world without having to rely on the server.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `features` | `[]protocol.GenerationFeature` | Features is a slice of all registered world generation features. |

### features 嵌套字段

- `name` (`string`)：Name is the name of the feature.
- `json` (`[]byte`)：JSON is the encoded JSON data instructing the client on how to generate the feature.

## 192 · ServerStats · 服务器统计

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：服务端发送的统计值；收到什么展示什么，不填假TPS。
- 本内核原始说明：ServerStats is a packet sent from the server to the client to update the client on server statistics. It is purely used for telemetry.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `server_time` | `float32` | ServerTime ... |
| `network_time` | `float32` | NetworkTime ... |

## 193 · RequestNetworkSettings · 请求网络设置

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：RequestNetworkSettings is sent by the client to request network settings, such as compression, from the server.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `client_protocol` | `int32` | ClientProtocol is the protocol version of the player. The player is disconnected if the protocol is incompatible with the protocol of the server. |

## 194 · GameTestRequest · 游戏测试请求

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：GameTestRequest ...

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `name` | `string` | Name represents the name of the test. |
| `rotation` | `uint8` | Rotation represents the rotation of the test. It is one of the constants above. |
| `repetitions` | `int32` | Repetitions represents the amount of times the test will be run. |
| `position` | `protocol.BlockPos` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position at which the test will be performed. |
| `stop_on_error` | `bool` | StopOnError indicates whether the test should immediately stop when an error is encountered. |
| `tests_per_row` | `int32` | TestsPerRow ... |
| `max_tests_per_batch` | `int32` | MaxTestsPerBatch ... |

## 195 · GameTestResults · 游戏测试结果

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：GameTestResults is a packet sent in response to the GameTestRequest packet, with a boolean indicating whether the test was successful or not, and an error string if the test failed.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `name` | `string` | Name represents the name of the test. |
| `succeeded` | `bool` | Succeeded indicates whether the test succeeded or not. |
| `error` | `string` | Error is the error that occurred. If Succeeded is true, this field is empty. |

## 196 · UpdateClientInputLocks · 更新客户端输入锁

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：UpdateClientInputLocks is sent by the server to the client to lock either the camera or physical movement of the client.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `locks` | `uint32` | Locks is a bitset that controls which locks are active. It is a combination of the constants above. If the camera is locked, then the player cannot change their pitch or yaw. If movement is locked, the player cannot move in any direction, they cannot jump, sneak or mount/dismount from any entities. |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the server's position of the client at the time the packet was sent. It is unclear what the exact purpose of this field is. |

## 197 · ClientCheatAbility · 客户端测试能力

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ClientCheatAbility functions the same as UpdateAbilities. It is unclear why these two were separated.  Deprecated: ClientCheatAbility is deprecated as of 1.20.10.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `ability_data` | `protocol.AbilityData` | AbilityData represents various data about the abilities of a player, such as ability layers or permissions. |

### ability_data 嵌套字段

- `entity_unique_id` (`int64`)：EntityUniqueID is a unique identifier of the player. It appears it is not required to fill this field out with a correct value. Simply writing 0 seems to work.
- `player_permissions` (`byte`)：PlayerPermissions is the permission level of the player as it shows up in the player list built up using the PlayerList packet.
- `command_permissions` (`byte`)：CommandPermissions is a set of permissions that specify what commands a player is allowed to execute.
- `layers` (`[]AbilityLayer`)：Layers contains all ability layers and their potential values. This should at least have one entry, being the base layer.
- `layers.type` (`uint16`)：Type represents the type of the layer. This is one of the AbilityLayerType constants defined above.
- `layers.abilities` (`uint32`)：Abilities is a set of abilities that are enabled for the layer. This is one of the Ability constants defined above.
- `layers.values` (`uint32`)：Values is a set of values that are associated with the enabled abilities, representing the values of the abilities.
- `layers.fly_speed` (`float32`)：FlySpeed is the default horizontal fly speed of the layer.
- `layers.vertical_fly_speed` (`float32`)：VerticalFlySpeed is the default vertical fly speed of the layer.
- `layers.walk_speed` (`float32`)：WalkSpeed is the default walk speed of the layer.

## 198 · CameraPresets · 摄像机预设

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：CameraPresets gives the client a list of custom camera presets.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `presets` | `[]protocol.CameraPreset` | Presets is a list of camera presets that can be used by other cameras. The order of this list is important because the index of presets is used as a pointer in the CameraInstruction packet. |

### presets 嵌套字段

- `name` (`string`)：Name is the name of the preset. Each preset must have their own unique name.
- `parent` (`string`)：Parent is the name of the preset that this preset extends upon. This can be left empty.
- `pos_x` (`Optional[float32]`)：PosX is the default X position of the camera.
- `pos_y` (`Optional[float32]`)：PosY is the default Y position of the camera.
- `pos_z` (`Optional[float32]`)：PosZ is the default Z position of the camera.
- `rot_x` (`Optional[float32]`)：RotX is the default pitch of the camera.
- `rot_y` (`Optional[float32]`)：RotY is the default yaw of the camera.
- `rotation_speed` (`Optional[float32]`)：RotationSpeed is the speed at which the camera should rotate.
- `snap_to_target` (`Optional[bool]`)：SnapToTarget determines whether the camera should snap to the target entity or not.
- `horizontal_rotation_limit` (`Optional[mgl32.Vec2]`)：HorizontalRotationLimit is the horizontal rotation limit of the camera.
- `vertical_rotation_limit` (`Optional[mgl32.Vec2]`)：VerticalRotationLimit is the vertical rotation limit of the camera.
- `continue_targeting` (`Optional[bool]`)：ContinueTargeting determines whether the camera should continue targeting when using aim assist.
- `tracking_radius` (`Optional[float32]`)：TrackingRadius is the radius around the camera that the aim assist should track targets.
- `view_offset` (`Optional[mgl32.Vec2]`)：ViewOffset is only used in a follow_orbit camera and controls an offset based on a pivot point to the player, causing it to be shifted in a certain direction.
- `entity_offset` (`Optional[mgl32.Vec3]`)：EntityOffset controls the offset from the entity that the camera should be rendered at.
- `radius` (`Optional[float32]`)：Radius is only used in a follow_orbit camera and controls how far away from the player the camera should be rendered.
- `min_yaw_limit` (`Optional[float32]`)：MinYawLimit is the minimum yaw limit of the camera.
- `max_yaw_limit` (`Optional[float32]`)：MaxYawLimit is the maximum yaw limit of the camera.
- `audio_listener` (`Optional[byte]`)：AudioListener defines where the audio should be played from when using this preset. This is one of the constants above.
- `player_effects` (`Optional[bool]`)：PlayerEffects is currently unknown.
- `aim_assist` (`Optional[CameraPresetAimAssist]`)：AimAssist defines the aim assist to use when using this preset.
- `control_scheme` (`Optional[byte]`)：ControlScheme is the control scheme that the client should use in this camera. It is one of the following: - ControlSchemeLockedPlayerRelativeStrafe is the default behaviour, this cannot be set when the client is in a custom camera. - ControlSchemeCameraRelative makes movement relative to the camera's transform, with the client's rotation being relative to the client's movement. - ControlSchemeCameraRelativeStrafe makes movement relative to the camera's transform, with the client's rotation being locked. - ControlSchemePlayerRelative makes movement relative to the player's transform, meaning holding left/right will make the player turn in a circle. - ControlSchemePlayerRelativeStrafe makes movement the same as the default behaviour, but can be used in a custom camera.

## 199 · UnlockedRecipes · 解锁配方

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：UnlockedRecipes gives the client a list of recipes that have been unlocked, restricting the recipes that appear in the recipe book.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `unlock_type` | `uint32` | UnlockType is the type of unlock that the packet represents, and can either be adding or removing a list of recipes. It is one of the constants listed above. |
| `recipes` | `[]string` | Recipes is a list of recipe names that have been unlocked. |

## 200 · PyRpc · 网易MsgPack远程事件

- 分组：网易与扩展
- 方向：双向，Value结构由具体事件约定
- 用法与边界：Value为真实MsgPack结构，OperationType依当前内核常量。ModEventC2S/S2C含什么由服务端实现；不是万能BDS权限，也不能凭空订阅未实现事件。
- 本内核原始说明：

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `value` | `interface {}` | Value from/to msgpack format |
| `operation_type` | `uint32` | OperationType is a fixed number |

## 203 · NeteaseJson · 网易JSON扩展

- 分组：网易与扩展
- 方向：按事件定义，需实测
- 用法与边界：真实字段见本内核定义；先抓取样本，不猜业务JSON。
- 本内核原始说明：Netease packet

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `data` | `[]uint8` | Netease: string field, but prased as byte slice for convenience |

## 232 · ConfirmSkin · 确认皮肤

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：Netease packet

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `player_data` | `[]protocol.NeteasePlayerData` | Netease: skin info |
| `uids` | `[]string` | Netease: launcher uids |
| `unknown3` | `[]string` | Netease |

### player_data 嵌套字段

- `unknown1` (`bool`)：Netease
- `uuid` (`uuid.UUID`)：Netease
- `unknown2` (`string`)：Netease

## 240 · SyncSkin · 同步皮肤

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：Netease Packet

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `player_data` | `[]protocol.NeteasePlayerData` | Netease: skin info |
| `player_uids` | `[]string` | Netease: player uids |
| `unknown` | `[]string` | Netease |
| `skin_item_ids` | `[]string` | Netease: skin item ids |
| `skin` | `protocol.Skin` | Netease: skin |

### player_data 嵌套字段

- `unknown1` (`bool`)：Netease
- `uuid` (`uuid.UUID`)：Netease
- `unknown2` (`string`)：Netease

### skin 嵌套字段

- `skin_id` (`string`)：SkinID is a unique ID produced for the skin, for example 'c18e65aa-7b21-4637-9b63-8ad63622ef01_Alex' for the default Alex skin.
- `play_fab_id` (`string`)：PlayFabID is the PlayFab ID produced for the skin. PlayFab is the company that hosts the Marketplace, skins and other related features from the game. This ID is the ID of the skin used to store the skin inside of PlayFab.
- `skin_resource_patch` (`[]byte`)：SkinResourcePatch is a JSON encoded object holding some fields that point to the geometry that the skin has. The JSON object that this holds specifies the way that the geometry of animations and the default skin of the player are combined.
- `skin_data` (`[]byte`)：SkinData is a byte slice of SkinImageWidth * SkinImageHeight bytes. It is an RGBA ordered byte representation of the skin pixels.
- `animations` (`[]SkinAnimation`)：Animations is a list of all animations that the skin has.
- `animations.image_data` (`[]byte`)：ImageData is a byte slice of ImageWidth * ImageHeight bytes. It is an RGBA ordered byte representation of the animation image pixels. The ImageData contains FrameCount images in it, which each represent one stage of the animation. The actual part of the skin that this field holds depends on the AnimationType, where SkinAnimationHead holds only the head and its hat, whereas the other animations hold the entire body of the skin.
- `animations.animation_type` (`uint32`)：AnimationType is the type of the animation, which is one of the types found above. The data that ImageData contains depends on this type.
- `animations.frame_count` (`float32`)：FrameCount is the amount of frames that the skin animation holds. The number of frames here is the amount of images that may be found in the ImageData field.
- `animations.expression_type` (`uint32`)：ExpressionType is the type of expression made by the skin, which is one the types found above.
- `cape_data` (`[]byte`)：CapeData is a byte slice of 64*32*4 bytes. It is a RGBA ordered byte representation of the cape colours, much like the SkinData.
- `skin_geometry` (`[]byte`)：SkinGeometry is a JSON encoded structure of the geometry data of a skin, containing properties such as bones, uv, pivot etc.
- `animation_data` (`[]byte`)：TODO: Find out what value AnimationData holds and when it does hold something.
- `geometry_data_engine_version` (`[]byte`)：GeometryDataEngineVersion ...
- `premium_skin` (`bool`)：PremiumSkin specifies if this is a skin that was purchased from the marketplace.
- `persona_skin` (`bool`)：PersonaSkin specifies if this is a skin that was created using the in-game skin creator.
- `persona_cape_on_classic_skin` (`bool`)：PersonaCapeOnClassicSkin specifies if the skin had a Persona cape (in-game skin creator cape) equipped on a classic skin.
- `primary_user` (`bool`)：PrimaryUser ...
- `cape_id` (`string`)：CapeID is a unique identifier that identifies the cape. It usually holds a UUID in it.
- `full_id` (`string`)：FullID is an ID that represents the skin in full. The actual functionality is unknown: The client does not seem to send a value for this.
- `skin_colour` (`string`)：SkinColour is a hex representation (including #) of the base colour of the skin. An example of the colour sent here is '#b37b62'.
- `arm_size` (`string`)：ArmSize is the size of the arms of the player's model. This is either 'wide' (generally for male skins) or 'slim' (generally for female skins).
- `persona_pieces` (`[]PersonaPiece`)：PersonaPieces is a list of all persona pieces that the skin is composed of.
- `persona_pieces.piece_id` (`string`)：PieceId is a UUID that identifies the piece itself, which is unique for each separate piece.
- `persona_pieces.piece_type` (`string`)：PieceType holds the type of the piece. Several types I was able to find immediately are listed below. - persona_skeleton - persona_body - persona_skin - persona_bottom - persona_feet - persona_top - persona_mouth - persona_hair - persona_eyes - persona_facial_hair
- `persona_pieces.pack_id` (`string`)：PackID is a UUID that identifies the pack that the persona piece belongs to.
- `persona_pieces.default` (`bool`)：Default specifies if the piece is one of the default pieces. This is true when the piece is one of those that a Steve or Alex skin have.
- `persona_pieces.product_id` (`string`)：ProductID is a UUID that identifies the piece when it comes to purchases. It is empty for pieces that have the 'Default' field set to true.
- `piece_tint_colours` (`[]PersonaPieceTintColour`)：PieceTintColours is a list of specific tint colours for (some of) the persona pieces found in the list above.
- `piece_tint_colours.piece_type` (`string`)：PieceType is the type of the persona skin piece that this tint colour concerns. The piece type must always be present in the persona pieces list, but not each piece type has a tint colour sent. Pieces that do have a tint colour that I was able to find immediately are listed below. - persona_mouth - persona_eyes - persona_hair
- `piece_tint_colours.colours` (`[]string`)：Colours is a list four colours written in hex notation (note, that unlike the SkinColour field in the ClientData struct, this is actually ARGB, not just RGB). The colours refer to different parts of the skin piece. The 'persona_eyes' may have the following colours: ["#ffa12722","#ff2f1f0f","#ff3aafd9","#0"] The first hex colour represents the tint colour of the iris, the second hex colour represents the eyebrows and the third represents the sclera. The fourth is #0 because there are only 3 parts of the persona_eyes skin piece.
- `trusted` (`bool`)：Trusted specifies if the skin is 'trusted'. No code should rely on this field, as any proxy or client can easily change it.
- `override_appearance` (`bool`)：OverrideAppearance specifies if the skin should override the player's skin that is equipped client-side. When false, the client will reject the skin and continue to use the skin that the player has equipped.

## 300 · CameraInstruction · 摄像机指令

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：CameraInstruction gives a custom camera specific instructions to operate.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `set` | `protocol.Optional[github.com/OmineDev/flowers-for-machines/core/minecraft/protocol.CameraInstructionSet]` | Set is a camera instruction that sets the camera to a specified preset. |
| `clear` | `protocol.Optional[bool]` | Clear can be set to true to clear all the current camera instructions. |
| `fade` | `protocol.Optional[github.com/OmineDev/flowers-for-machines/core/minecraft/protocol.CameraInstructionFade]` | Fade is a camera instruction that fades the screen to a specified colour. |
| `target` | `protocol.Optional[github.com/OmineDev/flowers-for-machines/core/minecraft/protocol.CameraInstructionTarget]` | Target is a camera instruction that targets a specific entity. |
| `remove_target` | `protocol.Optional[bool]` | RemoveTarget can be set to true to remove the current aim assist target. |

## 302 · TrimData · 饰纹数据

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：TrimData is sent by the server to the client when they first join the server. It contains a list of all the patterns and materials that can be applied via armour trims.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `patterns` | `[]protocol.TrimPattern` | Patterns is a list of patterns that can be applied to armour. Each pattern has its own style and texture that is defined through resource packs. |
| `materials` | `[]protocol.TrimMaterial` | Materials is a list of materials that can be applied to armour. These are mostly different ores that have different colours for more customization. |

### patterns 嵌套字段

- `item_name` (`string`)：ItemName is the identifier of the item that represents the pattern, for example 'minecraft:wayfinder_armor_trim_smithing_template'.
- `pattern_id` (`string`)：PatternID is the identifier of the pattern, for example, 'wayfinder'.

### materials 嵌套字段

- `material_id` (`string`)：MaterialID is the identifier of the material, for example 'netherite'.
- `colour` (`string`)：Colour is the colour code used for text formatting, for example '§j'.
- `item_name` (`string`)：ItemName is the identifier of the item that represents the material, for example, 'minecraft:netherite_ingot'.

## 303 · OpenSign · 打开告示牌

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：OpenSign is sent by the server to open a sign for editing. As of 1.19.80, the player can interact with a sign to edit the text on both sides instead of just the front.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `position` | `protocol.BlockPos` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position of the sign to edit. The client uses this position to get the data of the sign, including the existing text and formatting etc. |
| `front_side` | `bool` | FrontSide dictates whether the front side of the sign should be opened for editing. If false, the back side is assumed to be edited. |

## 304 · AgentAnimation · 代理动画

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：AgentAnimation is an Education Edition packet sent from the server to the client to make an agent perform an animation.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `animation` | `uint8` | Animation is the ID of the animation that the agent should perform. As of its implementation, there are no IDs that can be used in the regular client. |
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the entity. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |

## 305 · RefreshEntitlements · 刷新权益

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：网易内核扩展，业务语义需样本核对。

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|

## 306 · PlayerToggleCrafterSlotRequest · 切换合成器槽位请求

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：对合成器指定槽位启用/禁用的客户端请求。
- 本内核原始说明：PlayerToggleCrafterSlotRequest is sent by the client when it tries to toggle the state of a slot within a Crafter.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `pos_x` | `int32` | PosX is the X position of the Crafter that is being modified. |
| `pos_y` | `int32` | PosY is the Y position of the Crafter that is being modified. |
| `pos_z` | `int32` | PosZ is the Z position of the Crafter that is being modified. |
| `slot` | `uint8` | 槽位编号。 Slot is the index of the slot that was toggled. This should be a value between 0 and 8. |
| `disabled` | `bool` | Disabled is the new state of the slot. If true, the slot is disabled, if false, the slot is enabled. |

## 307 · SetPlayerInventoryOptions · 设置玩家库存选项

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：SetPlayerInventoryOptions is a bidirectional packet that can be used to update the inventory options of a player.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `left_inventory_tab` | `uint8` | LeftInventoryTab is the tab that is selected on the left side of the inventory. This is usually for the creative inventory. It is one of the InventoryLeftTab constants above. |
| `right_inventory_tab` | `uint8` | RightInventoryTab is the tab that is selected on the right side of the inventory. This is usually for the player's own inventory. It is one of the InventoryRightTab constants above. |
| `filtering` | `bool` | Filtering is whether the player has enabled the filtering between recipes they have unlocked or not. |
| `inventory_layout` | `uint8` | InventoryLayout is the layout of the inventory. It is one of the InventoryLayout constants above. |
| `crafting_layout` | `uint8` | CraftingLayout is the layout of the crafting inventory. It is one of the InventoryLayout constants above. |

## 308 · SetHud · 设置界面

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：SetHud is sent by the server to set the visibility of individual HUD elements on the client.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `elements` | `[]int32` | Elements is a list of HUD elements that are being modified. The values can be any of the HudElement constants above. |
| `visibility` | `int32` | Visibility represents the new visibility of the specified Elements. It can be any of the HudVisibility constants above. |

## 309 · AwardAchievement · 授予成就

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：AwardAchievement is sent by the server to award an achievement to a player.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `achievement_id` | `int32` | AchievementID is the ID of the achievement that should be awarded to the player. The values for these IDs are currently unknown. |

## 310 · ClientBoundCloseForm · 关闭客户端表单

- 分组：其他协议功能
- 方向：需按具体包及服务端样本核对
- 用法与边界：服务端要求关闭当前表单。
- 本内核原始说明：网易内核扩展，业务语义需样本核对。

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|

## 312 · ServerBoundLoadingScreen · 服务器定向加载屏幕

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ServerBoundLoadingScreen is sent by the client to tell the server about the state of the loading screen that the client is currently displaying.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `type` | `int32` | Type is the type of the loading screen event. It is one of the constants that may be found above. |
| `loading_screen_id` | `protocol.Optional[uint32]` | LoadingScreenID is the ID of the screen that was previously sent by the server in the ChangeDimension packet. The server should validate that the ID matches the last one it sent. |

## 313 · JigsawStructureData · 拼图结构数据

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：JigsawStructureData is sent by the server to let the client know all the rules for jigsaw structures.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `structure_data` | `[]uint8` | StructureData is a network NBT serialised compound of all the jigsaw structure rules defined on the server. |

## 314 · CurrentStructureFeature · 当前结构特征

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：同步当前结构特征名称或信息。
- 本内核原始说明：CurrentStructureFeature is sent by the server to let the client know the name of the structure feature that the player is currently occupying.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `current_feature` | `string` | CurrentFeature is the identifier of the structure feature that the player is currently occupying. If the player is not occupying any structure feature, this field is empty. |

## 315 · ServerBoundDiagnostics · 服务器定向诊断

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ServerBoundDiagnostics is sent by the client to tell the server about the performance diagnostics of the client. It is sent by the client roughly every 500ms or 10 in-game ticks when the "Creator > Enable Client Diagnostics" setting is enabled.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `average_frames_per_second` | `float32` | AverageFramesPerSecond is the average amount of frames per second that the client has been running at. |
| `average_server_sim_tick_time` | `float32` | AverageServerSimTickTime is the average time that the server spends simulating a single tick in milliseconds. |
| `average_client_sim_tick_time` | `float32` | AverageClientSimTickTime is the average time that the client spends simulating a single tick in milliseconds. |
| `average_begin_frame_time` | `float32` | AverageBeginFrameTime is the average time that the client spends beginning a frame in milliseconds. |
| `average_input_time` | `float32` | AverageInputTime is the average time that the client spends processing input in milliseconds. |
| `average_render_time` | `float32` | AverageRenderTime is the average time that the client spends rendering in milliseconds. |
| `average_end_frame_time` | `float32` | AverageEndFrameTime is the average time that the client spends ending a frame in milliseconds. |
| `average_remainder_time_percent` | `float32` | AverageRemainderTimePercent is the average percentage of time that the client spends on tasks that are not accounted for. |
| `average_unaccounted_time_percent` | `float32` | AverageUnaccountedTimePercent is the average percentage of time that the client spends on unaccounted tasks. |

## 316 · CameraAimAssist · 摄像机瞄准辅助

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：CameraAimAssist is sent by the server to the client to set up aim assist for the client's camera.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `preset` | `string` | Preset is the ID of the preset that has previously been defined in the CameraAimAssistPresets packet. |
| `angle` | `mgl32.Vec2` | Angle is the maximum angle around the playes's cursor that the aim assist should check for a target, if TargetMode is set to protocol.AimAssistTargetModeAngle. |
| `distance` | `float32` | Distance is the maximum distance from the player's cursor should check for a target, if TargetMode is set to protocol.AimAssistTargetModeDistance. |
| `target_mode` | `uint8` | TargetMode is the mode that the camera should use for detecting targets. This is currently one of protocol.AimAssistTargetModeAngle or protocol.AimAssistTargetModeDistance. |
| `action` | `uint8` | Action is the action that should be performed with the aim assist. This is one of the constants above. |

## 317 · ContainerRegistryCleanup · 容器注册表清理

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ContainerRegistryCleanup is sent by the server to trigger a client-side cleanup of the dynamic container registry.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `removed_containers` | `[]protocol.FullContainerName` | RemovedContainers is a list of protocol.FullContainerName's that should be removed from the client-side container registry. |

### removed_containers 嵌套字段

- `container_id` (`byte`)：ContainerID is the ID of the container that the slot was in.
- `dynamic_container_id` (`Optional[uint32]`)：DynamicContainerID is the ID of the container if it is dynamic. If the container is not dynamic, this field should be left empty. A non-optional value of 0 is assumed to be non-empty.

## 318 · MovementEffect · 移动效果

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：移动效果参数的服务端同步，不是其他玩家按键记录。
- 本内核原始说明：MovementEffect is sent by the server to the client to update specific movement effects to allow the client to predict its movement. For example, fireworks used during gliding will send this packet to tell the client the exact duration of the boost.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the entity. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `type` | `int32` | Type is the type of movement effect being updated. It is one of the constants found above. |
| `duration` | `int32` | Duration is the duration of the effect, measured in ticks. |
| `tick` | `uint64` | 输入/同步的游戏刻编号。 Tick is the server tick at which the packet was sent. It is used in relation to CorrectPlayerMovePrediction. |

## 320 · CameraAimAssistPresets · 摄像机瞄准辅助预设

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：CameraAimAssistPresets is sent by the server to the client to provide a list of categories and presets that can be used when sending a CameraAimAssist packet or a CameraInstruction including aim assist.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `categories` | `[]protocol.CameraAimAssistCategory` | Categories is a list of categories which can be referenced by one of the Presets. |
| `presets` | `[]protocol.CameraAimAssistPreset` | Presets is a list of presets which define a base for how aim assist should behave |
| `operation` | `uint8` | Operation is the operation to perform with the presets. It is one of the constants above. |

### categories 嵌套字段

- `name` (`string`)：Name is the name of the category which can be used by a CameraAimAssistPreset.
- `priorities` (`CameraAimAssistPriorities`)：Priorities represents the block and entity specific priorities as well as the default priorities for this category.
- `priorities.entities` (`[]CameraAimAssistPriority`)：Entities is a list of priorities for specific entity identifiers.
- `priorities.entities.identifier` (`string`)：Identifier is the identifier of a target to define the priority for.
- `priorities.entities.priority` (`int32`)：Priority is the priority for this specific target.
- `priorities.blocks` (`[]CameraAimAssistPriority`)：Blocks is a list of priorities for specific block identifiers.
- `priorities.blocks.identifier` (`string`)：Identifier is the identifier of a target to define the priority for.
- `priorities.blocks.priority` (`int32`)：Priority is the priority for this specific target.
- `priorities.entity_default` (`Optional[int32]`)：EntityDefault is the default priority for entities.
- `priorities.block_default` (`Optional[int32]`)：BlockDefault is the default priority for blocks.

### presets 嵌套字段

- `identifier` (`string`)：Identifier represents the identifier of this preset.
- `block_exclusions` (`[]string`)：BlockExclusions is a list of block identifiers that should be ignored by the aim assist.
- `liquid_targets` (`[]string`)：LiquidTargets is a list of entity identifiers that should be targetted when inside of a liquid.
- `item_settings` (`[]CameraAimAssistItemSettings`)：ItemSettings is a list of settings for specific item identifiers. If an item is not listed here, it will fallback to DefaultItemSettings or HandSettings if no item is held.
- `item_settings.item` (`string`)：Item is the identifier of the item to apply the settings to.
- `item_settings.category` (`string`)：Category is the identifier of a category to use which has been defined by a CameraAimAssistCategory. Only categories defined in the Categories slice used by the CameraAimAssistPreset can be used here.
- `default_item_settings` (`Optional[string]`)：DefaultItemSettings is the identifier of a category to use when the player is not holding an item listed in ItemSettings. This must be the identifier of a category within the Categories slice.
- `hand_settings` (`Optional[string]`)：HandSettings is the identifier of a category to use when the player is not holding an item. This must be the identifier of a category within Categories slice.

## 321 · ClientCameraAimAssist · 客户端摄像机瞄准辅助

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ClientCameraAimAssist is sent by the server to send a player animation from one player to all viewers of that player. It is used for a couple of actions, such as arm swimming and critical hits.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `preset_id` | `string` | PresetID is the identifier of the preset to use which was previously defined in the CameraAimAssistPresets packet. |
| `action` | `uint8` | Action is the action to perform with the aim assist. It is one of the constants above. |
| `allow_aim_assist` | `bool` | AllowAimAssist specifies the client can use aim assist or not. |

## 322 · ClientMovementPredictionSync · 客户端移动预测同步

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：客户端移动预测相关同步，按当前版本协议解释。
- 本内核原始说明：ClientMovementPredictionSync is sent by the client to the server periodically if the client has received movement corrections from the server, containing information about client-predictions that are relevant to movement.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `actor_flags` | `protocol.Bitset` | ActorFlags is a bitset of all the flags that are currently set for the client. |
| `bounding_box_scale` | `float32` | BoundingBoxScale is the scale of the client's bounding box. |
| `bounding_box_width` | `float32` | BoundingBoxWidth is the width of the client's bounding box. |
| `bounding_box_height` | `float32` | BoundingBoxHeight is the height of the client's bounding box. |
| `movement_speed` | `float32` | MovementSpeed is the movement speed attribute or 0 if not set. |
| `underwater_movement_speed` | `float32` | UnderwaterMovementSpeed is the underwater movement speed attribute or 0 if not set. |
| `lava_movement_speed` | `float32` | LavaMovementSpeed is the lava movement speed attribute or 0 if not set. |
| `jump_strength` | `float32` | JumpStrength is the jump strength attribute or 0 if not set. |
| `health` | `float32` | Health is the health attribute or 0 if not set. |
| `hunger` | `float32` | Hunger is the hunger attribute or 0 if not set. |
| `entity_unique_id` | `int64` | 实体唯一ID，与运行ID不同。 EntityUniqueID is the unique ID of the entity. The unique ID is a value that remains consistent across different sessions of the same world. |
| `flying` | `bool` | Flying specifies if the client is currently flying. |

### actor_flags 嵌套字段

- `size` (`int`)：
- `int` (`*big.Int`)：

## 323 · UpdateClientOptions · 更新客户端选项

- 分组：其他协议功能
- 方向：客户端→服务器
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：UpdateClientOptions is sent by the client when some of the client's options are updated, such as the graphics mode.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `graphics_mode` | `protocol.Optional[uint8]` | GraphicsMode is the graphics mode that the client is using. It is one of the constants above. |

## 324 · PlayerVideoCapture · 玩家视频捕获

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：PlayerVideoCapture packet is sent by the server to start or stop video recording for a player. This packet only works on development builds and has no effect on retail builds. When recording, the client will save individual frames to '/LocalCache/minecraftpe' in the format specified below.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `action` | `uint8` | Action is the action to perform with the video capture. It is one of the constants above. |
| `frame_rate` | `int32` | FrameRate is the frame rate at which the video should be recorded. It is only used when Action is PlayerVideoCaptureActionStart. A higher frame rate will cause more frames to be recorded, but also a noticeable increase in lag. |
| `file_prefix` | `string` | FilePrefix is the prefix of the file name that will be used to save the frames. The frames will be saved in the format 'FilePrefix%d.png' where %d is the frame index. |

## 325 · PlayerUpdateEntityOverrides · 玩家实体覆盖更新

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：实体属性覆盖数据的更新，具体字段按服务端样本核对。
- 本内核原始说明：PlayerUpdateEntityOverrides is sent by the server to modify an entity's properties individually.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `entity_runtime_id` | `uint64` | 当前会话实体运行ID，不同会话不能复用；64位ID建议传十进制字符串。 EntityRuntimeID is the runtime ID of the entity. The runtime ID is unique for each world session, and entities are generally identified in packets using this runtime ID. |
| `property_index` | `uint32` | PropertyIndex is the index of the property to modify. The index is unique for each property of an entity. |
| `type` | `uint8` | Type is the type of action to perform with the property. It is one of the constants above. |
| `int_value` | `int32` | IntValue is the new integer value of the property. It is only used when Type is set to PlayerUpdateEntityOverridesTypeInt. |
| `float_value` | `float32` | FloatValue is the new float value of the property. It is only used when Type is set to PlayerUpdateEntityOverridesTypeFloat. |

## 326 · PlayerLocation · 玩家位置

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：PlayerLocation is sent by the server to the client to either update a player's position on the locator bar, or remove them completely. The client will determine how to render the player on the locator bar based on their own distance to Position.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `type` | `int32` | Type is the action that is being performed. It is one of the constants above. |
| `entity_unique_id` | `int64` | 实体唯一ID，与运行ID不同。 EntityUniqueID is the unique ID of the entity. The unique ID is a value that remains consistent across different sessions of the same world. |
| `position` | `mgl32.Vec3` | 位置坐标，维度与有效性按本包上下文确定。 Position is the position of the player to be used on the locator bar. This is only set when the Type is PlayerLocationTypeCoordinates. |

## 327 · ClientBoundControlSchemeSet · 客户端定向控制方案设置

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：用途按本内核原始定义说明；不自动赋予额外游戏权限。
- 本内核原始说明：ClientBoundControlSchemeSet is sent by the server upon the client's request or the usage of the vanilla controlscheme command. It is used to set the control scheme of the client, often used in combination with custom cameras.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `control_scheme` | `uint8` | ControlScheme is the control scheme that the client should use. It is one of the following: - ControlSchemeLockedPlayerRelativeStrafe is the default behaviour, this cannot be set when the client is in a custom camera. - ControlSchemeCameraRelative makes movement relative to the camera's transform, with the client's rotation being relative to the client's movement. - ControlSchemeCameraRelativeStrafe makes movement relative to the camera's transform, with the client's rotation being locked. - ControlSchemePlayerRelative makes movement relative to the player's transform, meaning holding left/right will make the player turn in a circle. - ControlSchemePlayerRelativeStrafe makes movement the same as the default behaviour, but can be used in a custom camera. |

## 328 · ServerScriptDebugDrawer · 服务端脚本调试绘制

- 分组：其他协议功能
- 方向：服务器→客户端
- 用法与边界：脚本调试图形数据，需要服务端启用对应调试。
- 本内核原始说明：ServerScriptDebugDrawer is a packet sent by the server to instruct the client to render one or more debug shapes. Each packet fully replaces any previously rendered shapes. To remove a shape, omit it from the next packet's Shapes slice.

| 字段 | 当前类型 | 语义与约束 |
|---|---|---|
| `shapes` | `[]packet.DebugDrawerShape` | Shapes is a list of shapes to draw on the client-side. |

### shapes 嵌套字段

- `network_id` (`uint64`)：NetworkID is the network ID of the shape.
- `type` (`protocol.Optional[uint8]`)：Type is the type of the shape.
- `location` (`protocol.Optional[mgl32.Vec3]`)：Location is the location of the shape.
- `scale` (`protocol.Optional[float32]`)：Scale is the scale of the shape.
- `rotation` (`protocol.Optional[mgl32.Vec3]`)：Rotation is the rotation of the shape.
- `total_time_left` (`protocol.Optional[float32]`)：TotalTimeLeft is the total time left of the shape.
- `colour` (`protocol.Optional[color.RGBA]`)：Colour is the ARGB colour of the shape.
- `text` (`protocol.Optional[string]`)：Text is the text of the shape.
- `box_bound` (`protocol.Optional[mgl32.Vec3]`)：BoxBound is the box bound of the shape.
- `line_end_location` (`protocol.Optional[mgl32.Vec3]`)：LineEndLocation is the line end location of the shape.
- `arrow_head_length` (`protocol.Optional[float32]`)：ArrowHeadLength is the arrow head length of the shape.
- `arrow_head_radius` (`protocol.Optional[float32]`)：ArrowHeadRadius is the arrow head radius of the shape.
- `segments` (`protocol.Optional[byte]`)：Segments is the number of segments of the shape.

