# Mundo com um host

Um servidor de rede sozinho não executa BOTW: os monstros, a física, os scripts e o carregamento de regiões ainda vivem no motor do jogo. A proposta inicial é eleger um dos jogos como **host da simulação do mundo** e usar o servidor para ordenar e distribuir seus eventos. Cada cliente continua executando o jogo para renderizar e controlar seu próprio Link.

| Estado | Responsável proposto |
|---|---|
| Movimento e ações do próprio Link | Cliente proprietário, com validação do servidor |
| Vida e inventário pessoais | Jogador proprietário; regras e validação do servidor |
| Monstros e eventos compartilhados | Jogo host, com decisões publicadas pelo servidor |
| Identidade dos objetos, sequência e persistência dos eventos | Servidor |
| Entrada posterior na sessão | Snapshot do host + eventos posteriores, aplicados pelo cliente |

Isso evita duas IAs decidindo destinos diferentes para o mesmo monstro. Ainda exige hooks para capturar e aplicar cada evento: posição, alvo, dano, morte, objetos e coleta. Só copiar o save do host não fornece sincronização em tempo real e pode sobrescrever inventários individuais.

Para a próxima camada de mundo, implementar primeiro um monstro identificado pelo objeto do mapa: host publica posição/vida/alvo, cliente deixa de simular a IA daquele monstro e reproduz o estado. O servidor valida autoridade e sequência; um cliente que entra depois recebe o snapshot atual. Troca de host exige uma época nova e descarte de eventos da autoridade anterior. Essa troca ainda não existe para IA/eventos; a prévia 8 implementa épocas e sucessão somente para horário/clima.

**Estado da prévia 8:** há presença nativa confirmada pelo jogador, transporte de animação/equipamento, convergência inicial de vida de monstros e autoridade de horário/clima com sucessão. Não há host de simulação, IA compartilhada, mapa criado no servidor ou replay geral de eventos. Um servidor dedicado que execute o motor completo é uma etapa posterior, que também exige resolver execução sem renderização e carregamento de regiões sem jogador local.
