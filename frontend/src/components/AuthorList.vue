<template>
    <div>
        <div class="mb-4">
            <label for="letter">Выберите букву:</label>
            <select v-model="selectedLetter" @change="fetchAuthors">
                <option v-for="l in alphabet" :key="l" :value="l">{{ l }}</option>
            </select>
        </div>

        {{ authors }}

        <!-- <p >Нет авторов для выбранной буквы</p> -->
    </div>
</template>

<script>
export default {
    data() {
        return {
            authors: [],
            selectedLetter: 'Д',
            alphabet: 'АБВГДЕЖЗИЙКЛМНОПРСТУФХЦЧШЩЭЮЯABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('')
        }
    },
    mounted() {
        this.fetchAuthors()
    },
    methods: {
        async fetchAuthors() {
            try {
                fetch(`http://localhost:3001/api/authors?startsWith=${this.selectedLetter}`)
                    .then(res => {
                        if (!res.ok) throw new Error(res.statusText)
                        return res.text()
                    })
                    .then(data => {
                        console.log(data)
                        this.authors = data
                    })
            } catch (err) {
                console.error('Ошибка загрузки авторов:', err)
                this.authors = []
            }
        }
    }
}
</script>

<style scoped>
select {
    padding: 4px;
    margin-left: 8px;
}
</style>
