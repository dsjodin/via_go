import Link from "next/link";

const menuItems = [
    {
        title: "MENU",
        items: [
            { name: "Pools", link: "/pools" },
            { name: "Groups", link: "/groups" },
        ],
    },
];

const Menu = () => {
    return (
        <div className="mt-4 text-sm">
            {menuItems.map(i=>(
                <div className="flex flex-col gap-2" key={i.title}>
                    <span className="text-gray-400 font-light my-4">{i.title}</span>
                    {i.items.map(item=>(
                        <Link href={item.link} key={item.name} className="flex items-center justify-center lg:justify-start gap-4 text-gray-500 py-2">
                            <span>{item.name}</span>
                        </Link>
                    ))}
                </div>
            ))}
        </div>
    )
}

export default Menu