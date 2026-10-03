package nbt_item

import (
	"fmt"

	"github.com/OmineDev/flowers-for-machines/game_control/resources_control"
	nbt_assigner_interface "github.com/OmineDev/flowers-for-machines/nbt_assigner/interface"
	"github.com/OmineDev/flowers-for-machines/nbt_assigner/nbt_cache"
	"github.com/OmineDev/flowers-for-machines/nbt_assigner/nbt_console"
	nbt_parser_interface "github.com/OmineDev/flowers-for-machines/nbt_parser/interface"
	nbt_parser_item "github.com/OmineDev/flowers-for-machines/nbt_parser/item"
)

func init() {
	nbt_assigner_interface.MakeNBTItemMethod = MakeNBTItemMethod
	nbt_assigner_interface.EnchMultiple = EnchMultiple
	nbt_assigner_interface.RenameMultiple = RenameMultiple
	nbt_assigner_interface.EnchAndRenameMultiple = EnchAndRenameMultiple
}

// NBTItemIsSupported 检查 item 是否是受支持的复杂物品
func NBTItemIsSupported(item nbt_parser_interface.Item) bool {
	switch item.(type) {
	case *nbt_parser_item.Book:
	case *nbt_parser_item.Banner:
	case *nbt_parser_item.Shield:
	default:
		return false
	}
	return true
}

// MakeNBTItemMethod 根据传入的操作台、缓存命中系统和多个物品，
// 将它们归类为每种复杂物品。对于 result 中的每个元素，可以使用
// Make 制作它们
func MakeNBTItemMethod(
	console *nbt_console.Console,
	cache *nbt_cache.NBTCacheSystem,
	multipleItems ...nbt_parser_interface.Item,
) (result []nbt_assigner_interface.Item) {
	if len(multipleItems) == 0 {
		return nil
	}

	books := make([]nbt_parser_interface.Item, 0)
	banners := make([]nbt_parser_interface.Item, 0)
	shields := make([]nbt_parser_interface.Item, 0)

	for _, item := range multipleItems {
		switch item.(type) {
		case *nbt_parser_item.Book:
			books = append(books, item)
		case *nbt_parser_item.Banner:
			banners = append(banners, item)
		case *nbt_parser_item.Shield:
			shields = append(shields, item)
		}
	}

	if len(books) > 0 {
		element := &Book{api: console}
		element.Append(books...)
		result = append(result, element)
	}
	if len(banners) > 0 {
		element := &Banner{
			api:             console,
			maxSlotCanUse:   BannerMaxSlotCanUse,
			maxBannerToMake: BannerMaxBannerToMake,
		}
		element.Append(banners...)
		result = append(result, element)
	}
	if len(shields) > 0 {
		element := &Shield{api: console}
		element.Append(shields...)
		result = append(result, element)
	}

	return result
}

// EnchMultiple 根据操作台 console 和已放入背包的多个物品 multipleItems，
// 将它们进行一一附魔处理。应当说明的是，这些物品应当置于非快捷栏的物品栏，
// 并且对于无需处理的物品，应当简单的置为 nil
func EnchMultiple(
	console *nbt_console.Console,
	multipleItems [27]*nbt_parser_interface.Item,
) error {
	api := console.API()

	enchItems := make([]resources_control.SlotID, 0)
	enchItemsCount := make(map[resources_control.SlotID]uint8)

	for index, value := range multipleItems {
		if value == nil {
			continue
		}

		slotID := resources_control.SlotID(index + 9)
		defaultItem := (*value).UnderlyingItem().(*nbt_parser_item.DefaultItem)

		if len(defaultItem.Enhance.EnchList) > 0 {
			enchItems = append(enchItems, slotID)
			enchItemsCount[slotID] = defaultItem.ItemCount()
		}
	}

	if len(enchItems) > 0 {
		success, err := api.ContainerOpenAndClose().OpenInventory()
		if err != nil {
			return fmt.Errorf("多次附魔: %v", err)
		}
		if !success {
			return fmt.Errorf("多次附魔: 打开物品栏失败")
		}
		defer api.ContainerOpenAndClose().CloseContainer()
	}

	for {
		if len(enchItems) == 0 {
			break
		}

		currentRound := enchItems[0:min(len(enchItems), 9)]
		transaction := api.ItemStackOperation().OpenTransaction()

		for dstSlotID, srcSlotID := range currentRound {
			_ = transaction.MoveBetweenInventory(
				srcSlotID,
				resources_control.SlotID(dstSlotID),
				enchItemsCount[srcSlotID],
			)
		}

		success, _, _, err := transaction.Commit()
		if err != nil {
			return fmt.Errorf("多次附魔: %v", err)
		}
		if !success {
			return fmt.Errorf("多次附魔: 服务器拒绝了物品堆叠操作（附魔阶段1）")
		}

		for index, originSlotID := range currentRound {
			item := multipleItems[originSlotID-9]
			defaultItem := (*item).UnderlyingItem().(*nbt_parser_item.DefaultItem)

			currentSlotID := resources_control.SlotID(index)
			if console.HotbarSlotID() != currentSlotID {
				err = console.ChangeAndUpdateHotbarSlotID(currentSlotID)
				if err != nil {
					return fmt.Errorf("多次附魔: %v", err)
				}
			}

			for _, ench := range defaultItem.Enhance.EnchList {
				// 走网易 AICommand（PyRpc）通道：标准 SettingsCommand 包在网易上附魔可能静默失败，
				// 导致完整性检查 Self loop。AICommand 是导入使用的可靠通道。
				err = api.Commands().SendAICommand(fmt.Sprintf("enchant @s %d %d", ench.ID, ench.Level))
				if err != nil {
					return fmt.Errorf("多次附魔: %v", err)
				}
			}

			err = api.Commands().AwaitChangesGeneral()
			if err != nil {
				return fmt.Errorf("多次附魔: %v", err)
			}
		}

		for currentSlotID, originSlotID := range currentRound {
			_ = transaction.MoveBetweenInventory(
				resources_control.SlotID(currentSlotID),
				originSlotID,
				enchItemsCount[originSlotID],
			)
		}

		success, _, _, err = transaction.Commit()
		if err != nil {
			return fmt.Errorf("多次附魔: %v", err)
		}
		if !success {
			return fmt.Errorf("多次附魔: 服务器拒绝了物品堆叠操作（附魔阶段2）")
		}

		enchItems = enchItems[len(currentRound):]
	}

	return nil
}

// RenameMultiple 根据操作台 console 和已放入背包的多个物品 multipleItems，
// 将它们进行集中性物品改名处理。应当说明的是，这些物品应当置于非快捷栏的物品栏，
// 并且对于无需处理的物品，应当简单的置为 nil
func RenameMultiple(
	console *nbt_console.Console,
	multipleItems [27]*nbt_parser_interface.Item,
) error {
	api := console.API()

	renameItems := make([]resources_control.SlotID, 0)
	renameItemsNewName := make([]string, 0)

	for index, value := range multipleItems {
		if value == nil {
			continue
		}

		slotID := resources_control.SlotID(index + 9)
		defaultItem := (*value).UnderlyingItem().(*nbt_parser_item.DefaultItem)
		displayName := defaultItem.Enhance.DisplayName

		if len(displayName) > 0 {
			renameItems = append(renameItems, slotID)
			renameItemsNewName = append(renameItemsNewName, displayName)
		}
	}

	if len(renameItems) == 0 {
		return nil
	}

	index, err := console.FindOrGenerateNewAnvil()
	if err != nil {
		return fmt.Errorf("多次重命名: %v", err)
	}

	success, err := console.OpenContainerByIndex(index)
	if err != nil {
		return fmt.Errorf("多次重命名: %v", err)
	}
	if !success {
		return fmt.Errorf("多次重命名: 打开铁砧失败")
	}
	defer api.ContainerOpenAndClose().CloseContainer()

	transaction := api.ItemStackOperation().OpenTransaction()
	for index, slotID := range renameItems {
		_ = transaction.RenameInventoryItem(
			slotID,
			renameItemsNewName[index],
		)
	}

	success, _, _, err = transaction.Commit()
	if err != nil {
		return fmt.Errorf("多次重命名: %v", err)
	}
	if !success {
		return fmt.Errorf("多次重命名: 服务器拒绝了重命名操作")
	}

	return nil
}

// EnchAndRenameMultiple 根据操作台 console 和已放入背包的多个物品 multipleItems，
// 将它们进行集中性的物品附魔和物品改名处理。应当说明的是，这些物品应当置于非快捷栏的物品栏，
// 并且对于无需处理的物品，应当简单的置为 nil
func EnchAndRenameMultiple(
	console *nbt_console.Console,
	multipleItems [27]*nbt_parser_interface.Item,
) error {
	err := EnchMultiple(console, multipleItems)
	if err != nil {
		return fmt.Errorf("附魔并多次重命名: %v", err)
	}
	err = RenameMultiple(console, multipleItems)
	if err != nil {
		return fmt.Errorf("附魔并多次重命名: %v", err)
	}
	return nil
}
